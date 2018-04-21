/*
 * Chain Query
 *
 * The LBRY blockchain is read into SQL where important structured information can be extracted through the Chain Query API.
 *
 * API version: 0.1.0
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package swagger

// Contains the name and number of rows for a table.
type TableSize struct {

	// Name of the table being referenced.
	TableName string `json:"TableName,omitempty"`

	// The number of rows in the referenced table
	NrRows int64 `json:"NrRows,omitempty"`
}
