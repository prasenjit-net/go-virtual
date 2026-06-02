# go-virtual — Claude Code guidance

## Storage backends

There are three storage backends that must all behave identically:
- `internal/storage/file.go` — file-based (loads everything into an in-memory layer, queries are Go loops)
- `internal/storage/memory.go` — pure in-memory
- `internal/storage/mongo.go` — MongoDB

**Whenever you add or change a storage feature, verify it works for all three backends.**

### The MongoDB field-promotion rule

MongoDB queries filter against top-level BSON document fields. `mongo.go` wraps every entity in a `genericDoc` where the entity is stored as a JSON blob in the `data` field — opaque to MongoDB. Any field used in a `bson.M{...}` filter **must be explicitly promoted** to a top-level field in `genericDoc` and populated on every write.

File/memory storage never hits this: they query Go struct fields directly, so a missing promotion is invisible until you test with Mongo.

Checklist when writing a new Mongo query:
1. Is the filter field present as a `bson:"..."` tag in `genericDoc`?
2. Is it set in every `marshalDoc` call (or explicitly after) for the relevant entity type?
3. Is there an index for it in `EnsureIndexes`?

Current promoted fields in `genericDoc`:

| BSON field           | Set from                       | Used by                                      |
|----------------------|--------------------------------|----------------------------------------------|
| `spec_id`            | `op.SpecID` / `binding.SpecID` | operations, bindings                         |
| `operation_id`       | `cfg.OperationID` / `binding.OperationID` | responses, bindings               |
| `script_id`          | `binding.ScriptID`             | bindings                                     |
| `response_config_id` | `binding.ResponseConfigID`     | bindings                                     |
| `source`             | `script.Source`                | scripts (excluded from JSON via `json:"-"`)  |

### Index management

Indexes live in `(*MongoStorage).EnsureIndexes` and are created by `go-virtual init`, **not** at server startup (the runtime user may lack `createIndex` privilege). Add a new index there whenever you promote a new query field.
