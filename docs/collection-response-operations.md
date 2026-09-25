# Collection response mapper operations

The main mapper supports Find One, Find Many, and Update. Additional mappers support Insert, Find One, Find Many, Update, Upsert, and Delete.

## Request order

1. Evaluate response priority, enabled state, tags, and explicit conditions.
2. Run the main query. An Update main mapper uses Find One with its query filters during selection, without writing anything.
3. Once selected, execute the main Update against that exact document and retain its updated result.
4. Execute additional mappers in the order shown in the editor.
5. Render the response from the main result, additional outputs, and field overrides.

For example, a main Update can query `status = pending` and set `status = confirmed`. The confirmed document supplies the response body even though it no longer matches the original query. Selection is not repeated after an update.

## Main operation

In the collection response editor's Query section, choose **Update**, enter the query filters, and add the data fields to write. Update affects one document and requires an object response root. Find Many requires an array root. Automatic preserves the existing behavior of deriving Find One or Find Many from the response shape.

Data fields can read request path, query, header, JSON body, literal values, or the selected primary document. Main Update's primary bindings read the document before the update.

An empty query result normally falls through to the next response. With `matchOnEmpty`, an empty Update response can be selected, performs no update, and renders null. It never implicitly inserts a document.

## Additional operations

| Mode | Filters | Data fields | Output |
| --- | --- | --- | --- |
| Find One | Optional | None | First matching document or null |
| Find Many | Optional | None | Matching documents as an array |
| Insert | None | Required | Inserted document, including its ID |
| Update | Required | Required | First matching document after changes, or null |
| Upsert | Required | Required | Updated document, or a newly inserted document |
| Delete | Required | None | Removed document before deletion, or null |

Each output is available under its mapper output key through a **Mapper output** field override. Outputs are plain documents, arrays, or null, without pipeline `_status` fields. Each mapper executes once per selected response, including when the main mapper returns an array.

Use the up/down controls to order operations. A Find One after an Upsert sees its write; a Find One after Delete no longer finds the deleted document. An audit Insert after a main Update can use primary bindings to record the updated values. A Delete output can supply fields from the removed document through an override.

Additional primary bindings read the main mapper's result after its Update. They require an object root. Later additional writes do not replace the main result snapshot; use an additional mapper's output in an override when that result should appear in the body. Bindings between additional mappers are not supported.

## Values, previews, and failures

Literal and body values preserve JSON types, including false, zero, null, objects, and arrays. Writes merge top-level fields; replace a whole nested object to change it. Dotted write targets and changing `_id` in Update/Upsert data are rejected. Missing write source values are errors, distinct from explicit null.

Preview performs reads only. It shows the main document before Update, skips additional mutations, and reports warnings. Saving and validating a configuration do not execute any operations.

Writes are session scoped and leave base collection data unchanged. An additional Update/Delete with no match is a successful no-op. Execution errors return a runtime error without selecting a fallback or retrying writes. Operations are not transactional: earlier writes remain if a later mapper or rendering fails. The request trace retains the main query attempt and completed/failed mapper diagnostics, and the response returns the session header when a write created a session.

Existing configurations remain valid: an omitted primary mode retains shape-derived reads, and existing additional Find One/Find Many mappers retain their behavior.
