package messages

//go:generate go run ../gen ../../../suite/messages.json catalog_gen.go messages
//go:generate go run ../gen ../../../suite/messages-queries.json queries_gen.go messages sourceQueriesJSON
//go:generate go run ../gen ../../../suite/messages-parse.json parse_gen.go messages sourceParseJSON
//go:generate go run ../gen ../../../suite/messages-check.json check_gen.go messages sourceCheckJSON
