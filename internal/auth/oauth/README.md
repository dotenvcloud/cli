# OAuth Implementation Notes

## ID Type Handling

The Laravel API returns numeric IDs (integers) for users and organizations, while the Go client needs to handle these appropriately:

- User IDs: `int64` in Go, numeric in Laravel
- Organization IDs: `int64` in Go, numeric in Laravel
- Organization slugs: Laravel uses `ulid` field, mapped to `Slug` in Go structs

### Design Decision

We chose to have the Go client adapt to the Laravel API's natural types rather than forcing the API to return strings for numeric data. This follows the principle that the API should define the contract, and clients should adapt to it.

### JSON Mapping

```go
type Organization struct {
    ID   int64  `json:"id"`       // Numeric ID from Laravel
    Slug string `json:"ulid"`     // Laravel's ulid field mapped to Slug
    Name string `json:"name"`
}
```

This approach:
- Keeps the Laravel API idiomatic (numeric IDs)
- Makes the Go client properly handle the API contract
- Avoids unnecessary type conversions on the server
- Maintains type safety in both languages