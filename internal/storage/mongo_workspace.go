//go:build !unit

package storage

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/prasenjit/go-virtual/internal/models"
)

// ApplySpecWorkspace writes every scoped entity inside one Mongo transaction.
// Mongo deployments without transaction support fail the save without applying
// a partial designer snapshot.
func (m *MongoStorage) ApplySpecWorkspace(w *models.SpecWorkspace) error {
	session, err := m.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(context.Background())
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		return nil, m.applyWorkspaceInTransaction(sc, w)
	}, options.Transaction())
	if err != nil {
		return fmt.Errorf("apply spec workspace transaction: %w", err)
	}
	return nil
}

func (m *MongoStorage) applyWorkspaceInTransaction(ctx context.Context, w *models.SpecWorkspace) error {
	put := func(collection string, id, specID, operationID, responseID string, value any) error {
		doc, err := marshalDoc(id, specID, operationID, responseID, value)
		if err != nil {
			return err
		}
		_, err = m.col(collection).ReplaceOne(ctx, bson.M{"_id": id}, doc, options.Replace().SetUpsert(true))
		return err
	}
	if err := put(colSpecs, w.Spec.ID, "", "", "", w.Spec); err != nil {
		return err
	}
	opIDs := make([]string, 0, len(w.Operations))
	responseIDs := make([]string, 0)
	for _, op := range w.Operations {
		opIDs = append(opIDs, op.Operation.ID)
		if err := put(colOperations, op.Operation.ID, op.Operation.SpecID, "", "", op.Operation); err != nil {
			return err
		}
		for _, response := range op.Responses {
			responseIDs = append(responseIDs, response.Response.ID)
		}
	}
	if len(opIDs) > 0 {
		cursor, err := m.col(colResponses).Find(ctx, bson.M{"operation_id": bson.M{"$in": opIDs}})
		if err != nil {
			return err
		}
		for cursor.Next(ctx) {
			var old genericDoc
			if err := cursor.Decode(&old); err != nil {
				cursor.Close(ctx)
				return err
			}
			responseIDs = append(responseIDs, old.ID)
		}
		if err := cursor.Err(); err != nil {
			cursor.Close(ctx)
			return err
		}
		if err := cursor.Close(ctx); err != nil {
			return err
		}
		if _, err := m.col(colResponses).DeleteMany(ctx, bson.M{"operation_id": bson.M{"$in": opIDs}}); err != nil {
			return err
		}
	}
	if err := m.deleteWorkspaceChildren(ctx, w.Spec.ID, opIDs, responseIDs); err != nil {
		return err
	}
	insert := func(collection, id, specID, operationID, responseID string, value any) error {
		doc, err := marshalDoc(id, specID, operationID, responseID, value)
		if err != nil {
			return err
		}
		_, err = m.col(collection).InsertOne(ctx, doc)
		return err
	}
	for _, binding := range w.SpecScripts {
		if err := insert(colBindings, binding.ID, binding.SpecID, "", "", binding); err != nil {
			return err
		}
	}
	for _, mapping := range w.SpecMappings {
		if err := insert(colCollectionMappings, mapping.ID, mapping.SpecID, "", "", mapping); err != nil {
			return err
		}
	}
	for _, rule := range w.SpecValidations {
		if err := insert(colValidations, rule.ID, rule.SpecID, "", "", rule); err != nil {
			return err
		}
	}
	for _, op := range w.Operations {
		for _, binding := range op.Scripts {
			if err := insert(colBindings, binding.ID, "", binding.OperationID, "", binding); err != nil {
				return err
			}
		}
		for _, mapping := range op.Mappings {
			if err := insert(colCollectionMappings, mapping.ID, "", mapping.OperationID, "", mapping); err != nil {
				return err
			}
		}
		for _, rule := range op.Validations {
			if err := insert(colValidations, rule.ID, "", rule.OperationID, "", rule); err != nil {
				return err
			}
		}
		for _, response := range op.Responses {
			if err := insert(colResponses, response.Response.ID, "", response.Response.OperationID, "", response.Response); err != nil {
				return err
			}
			for _, binding := range response.Scripts {
				if err := insert(colBindings, binding.ID, "", "", binding.ResponseConfigID, binding); err != nil {
					return err
				}
			}
			for _, mapping := range response.Mappings {
				if err := insert(colCollectionMappings, mapping.ID, "", "", mapping.ResponseConfigID, mapping); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *MongoStorage) deleteWorkspaceChildren(ctx context.Context, specID string, operationIDs, responseIDs []string) error {
	for _, collection := range []string{colBindings, colCollectionMappings, colValidations} {
		filters := bson.A{bson.M{"spec_id": specID}}
		if len(operationIDs) > 0 {
			filters = append(filters, bson.M{"operation_id": bson.M{"$in": operationIDs}})
		}
		if len(responseIDs) > 0 {
			filters = append(filters, bson.M{"response_config_id": bson.M{"$in": responseIDs}})
		}
		if _, err := m.col(collection).DeleteMany(ctx, bson.M{"$or": filters}); err != nil {
			return err
		}
	}
	return nil
}
