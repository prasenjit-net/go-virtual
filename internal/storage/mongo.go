//go:build !unit

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/prasenjit/go-virtual/internal/config"
	"github.com/prasenjit/go-virtual/internal/models"
)

// MongoStorage implements the Storage interface using MongoDB as the backend.
// Each entity type is stored in a dedicated collection. Models are serialised
// to/from JSON so that all json struct tags are honoured without duplicating
// field definitions in separate BSON document types.
type MongoStorage struct {
	client *mongo.Client
	db     *mongo.Database
	prefix string
}

// collection names (without prefix)
const (
	colSpecs         = "specs"
	colOperations    = "operations"
	colResponses     = "responses"
	colScripts       = "scripts"
	colAIScenarios   = "ai_scenarios"
	colBindings      = "script_bindings"
	colTags          = "tags"
)

// genericDoc is the BSON wrapper stored for each entity.
// _id holds the entity's natural key; data holds the JSON-encoded model.
type genericDoc struct {
	ID      string `bson:"_id"`
	Data    string `bson:"data"`
	// Relationship fields for indexed queries:
	SpecID      string `bson:"spec_id,omitempty"`
	OperationID string `bson:"operation_id,omitempty"`
	ScriptID    string `bson:"script_id,omitempty"`
	Enabled     *bool  `bson:"enabled,omitempty"`
}

// NewMongoStorage connects to MongoDB using the supplied config and returns a
// fully-initialised MongoStorage.
func NewMongoStorage(cfg config.MongoConfig) (*MongoStorage, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongo storage: URI must not be empty")
	}
	timeout := time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	opts := options.Client().ApplyURI(cfg.URI).SetConnectTimeout(timeout)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo storage: connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo storage: ping: %w", err)
	}

	s := &MongoStorage{
		client: client,
		db:     client.Database(cfg.Database),
		prefix: cfg.CollectionPrefix,
	}
	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo storage: ensure indexes: %w", err)
	}
	return s, nil
}

func (m *MongoStorage) col(name string) *mongo.Collection {
	return m.db.Collection(m.prefix + name)
}

func (m *MongoStorage) ensureIndexes(ctx context.Context) error {
	type indexSpec struct {
		col   string
		field string
	}
	indexes := []indexSpec{
		{colOperations, "spec_id"},
		{colResponses, "operation_id"},
		{colBindings, "operation_id"},
		{colBindings, "script_id"},
	}
	for _, idx := range indexes {
		model := mongo.IndexModel{
			Keys: bson.D{{Key: idx.field, Value: 1}},
		}
		if _, err := m.col(idx.col).Indexes().CreateOne(ctx, model); err != nil {
			return fmt.Errorf("create index %s.%s: %w", idx.col, idx.field, err)
		}
	}
	return nil
}

// --- helpers ----------------------------------------------------------------

func marshalDoc(id, specID, operationID, scriptID string, v any) (*genericDoc, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	doc := &genericDoc{
		ID:   id,
		Data: string(data),
	}
	if specID != "" {
		doc.SpecID = specID
	}
	if operationID != "" {
		doc.OperationID = operationID
	}
	if scriptID != "" {
		doc.ScriptID = scriptID
	}
	return doc, nil
}

func unmarshalDoc(doc *genericDoc, dest any) error {
	return json.Unmarshal([]byte(doc.Data), dest)
}

func ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// --- Spec operations --------------------------------------------------------

func (m *MongoStorage) CreateSpec(spec *models.Spec) error {
	doc, err := marshalDoc(spec.ID, "", "", "", spec)
	if err != nil {
		return fmt.Errorf("mongo: marshal spec: %w", err)
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = m.col(colSpecs).InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("mongo: insert spec: %w", err)
	}
	return nil
}

func (m *MongoStorage) GetSpec(id string) (*models.Spec, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	var doc genericDoc
	err := m.col(colSpecs).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("spec not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("mongo: get spec: %w", err)
	}
	var spec models.Spec
	if err := unmarshalDoc(&doc, &spec); err != nil {
		return nil, fmt.Errorf("mongo: decode spec: %w", err)
	}
	return &spec, nil
}

func (m *MongoStorage) GetAllSpecs() ([]*models.Spec, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colSpecs).Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("mongo: find specs: %w", err)
	}
	return decodeSpecs(cursor, ctx)
}

func (m *MongoStorage) GetEnabledSpecs() ([]*models.Spec, error) {
	// We filter in Go after decoding since "enabled" is embedded in data JSON.
	specs, err := m.GetAllSpecs()
	if err != nil {
		return nil, err
	}
	out := specs[:0]
	for _, s := range specs {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out, nil
}

func decodeSpecs(cursor *mongo.Cursor, ctx context.Context) ([]*models.Spec, error) {
	defer cursor.Close(ctx)
	var specs []*models.Spec
	for cursor.Next(ctx) {
		var doc genericDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo: decode spec doc: %w", err)
		}
		var spec models.Spec
		if err := unmarshalDoc(&doc, &spec); err != nil {
			return nil, fmt.Errorf("mongo: decode spec: %w", err)
		}
		specs = append(specs, &spec)
	}
	return specs, cursor.Err()
}

func (m *MongoStorage) UpdateSpec(spec *models.Spec) error {
	doc, err := marshalDoc(spec.ID, "", "", "", spec)
	if err != nil {
		return fmt.Errorf("mongo: marshal spec: %w", err)
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	res, err := m.col(colSpecs).ReplaceOne(ctx, bson.M{"_id": spec.ID}, doc)
	if err != nil {
		return fmt.Errorf("mongo: update spec: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("spec not found: %s", spec.ID)
	}
	return nil
}

func (m *MongoStorage) DeleteSpec(id string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colSpecs).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// --- Tag operations ---------------------------------------------------------

func (m *MongoStorage) ListTags() ([]*models.Tag, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colTags).Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("mongo: find tags: %w", err)
	}
	defer cursor.Close(ctx)
	var tags []*models.Tag
	for cursor.Next(ctx) {
		var doc genericDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var tag models.Tag
		if err := unmarshalDoc(&doc, &tag); err != nil {
			return nil, err
		}
		tags = append(tags, &tag)
	}
	return tags, cursor.Err()
}

func (m *MongoStorage) GetTag(name string) (*models.Tag, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	var doc genericDoc
	err := m.col(colTags).FindOne(ctx, bson.M{"_id": name}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("tag not found: %s", name)
	}
	if err != nil {
		return nil, err
	}
	var tag models.Tag
	if err := unmarshalDoc(&doc, &tag); err != nil {
		return nil, err
	}
	return &tag, nil
}

func (m *MongoStorage) CreateTag(tag *models.Tag) error {
	doc, err := marshalDoc(tag.Name, "", "", "", tag)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = m.col(colTags).InsertOne(ctx, doc)
	return err
}

func (m *MongoStorage) UpdateTag(oldName string, tag *models.Tag) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	if oldName != tag.Name {
		// Rename: delete old, insert new.
		if _, err := m.col(colTags).DeleteOne(ctx, bson.M{"_id": oldName}); err != nil {
			return err
		}
	}
	doc, err := marshalDoc(tag.Name, "", "", "", tag)
	if err != nil {
		return err
	}
	opts := options.Replace().SetUpsert(true)
	_, err = m.col(colTags).ReplaceOne(ctx, bson.M{"_id": tag.Name}, doc, opts)
	return err
}

func (m *MongoStorage) DeleteTag(name string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colTags).DeleteOne(ctx, bson.M{"_id": name})
	return err
}

// --- Operation operations ---------------------------------------------------

func (m *MongoStorage) CreateOperation(op *models.Operation) error {
	doc, err := marshalDoc(op.ID, op.SpecID, "", "", op)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = m.col(colOperations).InsertOne(ctx, doc)
	return err
}

func (m *MongoStorage) GetOperation(id string) (*models.Operation, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	var doc genericDoc
	err := m.col(colOperations).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("operation not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var op models.Operation
	if err := unmarshalDoc(&doc, &op); err != nil {
		return nil, err
	}
	return &op, nil
}

func (m *MongoStorage) GetOperationsBySpec(specID string) ([]*models.Operation, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colOperations).Find(ctx, bson.M{"spec_id": specID})
	if err != nil {
		return nil, err
	}
	return decodeOperations(cursor, ctx)
}

func (m *MongoStorage) GetAllOperations() ([]*models.Operation, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colOperations).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	return decodeOperations(cursor, ctx)
}

func decodeOperations(cursor *mongo.Cursor, ctx context.Context) ([]*models.Operation, error) {
	defer cursor.Close(ctx)
	var ops []*models.Operation
	for cursor.Next(ctx) {
		var doc genericDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var op models.Operation
		if err := unmarshalDoc(&doc, &op); err != nil {
			return nil, err
		}
		ops = append(ops, &op)
	}
	return ops, cursor.Err()
}

func (m *MongoStorage) UpdateOperation(op *models.Operation) error {
	doc, err := marshalDoc(op.ID, op.SpecID, "", "", op)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	opts := options.Replace().SetUpsert(true)
	_, err = m.col(colOperations).ReplaceOne(ctx, bson.M{"_id": op.ID}, doc, opts)
	return err
}

func (m *MongoStorage) DeleteOperation(id string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colOperations).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (m *MongoStorage) DeleteOperationsBySpec(specID string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colOperations).DeleteMany(ctx, bson.M{"spec_id": specID})
	return err
}

// --- ResponseConfig operations ----------------------------------------------

func (m *MongoStorage) CreateResponseConfig(cfg *models.ResponseConfig) error {
	doc, err := marshalDoc(cfg.ID, "", cfg.OperationID, "", cfg)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = m.col(colResponses).InsertOne(ctx, doc)
	return err
}

func (m *MongoStorage) GetResponseConfig(id string) (*models.ResponseConfig, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	var doc genericDoc
	err := m.col(colResponses).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("response config not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var rc models.ResponseConfig
	if err := unmarshalDoc(&doc, &rc); err != nil {
		return nil, err
	}
	return &rc, nil
}

func (m *MongoStorage) GetResponseConfigsByOperation(opID string) ([]*models.ResponseConfig, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colResponses).Find(ctx, bson.M{"operation_id": opID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var configs []*models.ResponseConfig
	for cursor.Next(ctx) {
		var doc genericDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var rc models.ResponseConfig
		if err := unmarshalDoc(&doc, &rc); err != nil {
			return nil, err
		}
		configs = append(configs, &rc)
	}
	return configs, cursor.Err()
}

func (m *MongoStorage) UpdateResponseConfig(cfg *models.ResponseConfig) error {
	doc, err := marshalDoc(cfg.ID, "", cfg.OperationID, "", cfg)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	opts := options.Replace().SetUpsert(true)
	_, err = m.col(colResponses).ReplaceOne(ctx, bson.M{"_id": cfg.ID}, doc, opts)
	return err
}

func (m *MongoStorage) DeleteResponseConfig(id string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colResponses).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (m *MongoStorage) DeleteResponseConfigsByOperation(opID string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colResponses).DeleteMany(ctx, bson.M{"operation_id": opID})
	return err
}

// --- Script operations ------------------------------------------------------

func (m *MongoStorage) CreateScript(script *models.Script) error {
	doc, err := marshalDoc(script.ID, "", "", "", script)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = m.col(colScripts).InsertOne(ctx, doc)
	return err
}

func (m *MongoStorage) GetScript(id string) (*models.Script, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	var doc genericDoc
	err := m.col(colScripts).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("script not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var script models.Script
	if err := unmarshalDoc(&doc, &script); err != nil {
		return nil, err
	}
	return &script, nil
}

func (m *MongoStorage) GetAllScripts() ([]*models.Script, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colScripts).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var scripts []*models.Script
	for cursor.Next(ctx) {
		var doc genericDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var script models.Script
		if err := unmarshalDoc(&doc, &script); err != nil {
			return nil, err
		}
		scripts = append(scripts, &script)
	}
	return scripts, cursor.Err()
}

func (m *MongoStorage) UpdateScript(script *models.Script) error {
	doc, err := marshalDoc(script.ID, "", "", "", script)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	opts := options.Replace().SetUpsert(true)
	_, err = m.col(colScripts).ReplaceOne(ctx, bson.M{"_id": script.ID}, doc, opts)
	return err
}

func (m *MongoStorage) DeleteScript(id string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colScripts).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// --- AIScenario operations --------------------------------------------------

func (m *MongoStorage) ListAIScenarios() ([]*models.AIScenario, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colAIScenarios).Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var scenarios []*models.AIScenario
	for cursor.Next(ctx) {
		var doc genericDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var s models.AIScenario
		if err := unmarshalDoc(&doc, &s); err != nil {
			return nil, err
		}
		scenarios = append(scenarios, &s)
	}
	return scenarios, cursor.Err()
}

func (m *MongoStorage) GetAIScenario(id string) (*models.AIScenario, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	var doc genericDoc
	err := m.col(colAIScenarios).FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("ai scenario not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var s models.AIScenario
	if err := unmarshalDoc(&doc, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (m *MongoStorage) CreateAIScenario(scenario *models.AIScenario) error {
	doc, err := marshalDoc(scenario.ID, "", "", "", scenario)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = m.col(colAIScenarios).InsertOne(ctx, doc)
	return err
}

func (m *MongoStorage) UpdateAIScenario(scenario *models.AIScenario) error {
	doc, err := marshalDoc(scenario.ID, "", "", "", scenario)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	opts := options.Replace().SetUpsert(true)
	_, err = m.col(colAIScenarios).ReplaceOne(ctx, bson.M{"_id": scenario.ID}, doc, opts)
	return err
}

func (m *MongoStorage) DeleteAIScenario(id string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colAIScenarios).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// --- ScriptBinding operations -----------------------------------------------

func (m *MongoStorage) GetScriptBindings(operationID string) ([]*models.ScriptBinding, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	cursor, err := m.col(colBindings).Find(ctx, bson.M{"operation_id": operationID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var bindings []*models.ScriptBinding
	for cursor.Next(ctx) {
		var doc genericDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		var b models.ScriptBinding
		if err := unmarshalDoc(&doc, &b); err != nil {
			return nil, err
		}
		bindings = append(bindings, &b)
	}
	return bindings, cursor.Err()
}

func (m *MongoStorage) CreateScriptBinding(binding *models.ScriptBinding) error {
	doc, err := marshalDoc(binding.ID, "", binding.OperationID, binding.ScriptID, binding)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err = m.col(colBindings).InsertOne(ctx, doc)
	return err
}

func (m *MongoStorage) UpdateScriptBinding(binding *models.ScriptBinding) error {
	doc, err := marshalDoc(binding.ID, "", binding.OperationID, binding.ScriptID, binding)
	if err != nil {
		return err
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	opts := options.Replace().SetUpsert(true)
	_, err = m.col(colBindings).ReplaceOne(ctx, bson.M{"_id": binding.ID}, doc, opts)
	return err
}

func (m *MongoStorage) DeleteScriptBinding(id string) error {
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colBindings).DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (m *MongoStorage) DeleteScriptBindingsByScript(scriptID string) error {
	if scriptID == "" {
		return nil
	}
	ctx, cancel := ctxTimeout()
	defer cancel()
	_, err := m.col(colBindings).DeleteMany(ctx, bson.M{"script_id": scriptID})
	return err
}

// --- Utility ----------------------------------------------------------------

func (m *MongoStorage) Close() error {
	return m.client.Disconnect(context.Background())
}
