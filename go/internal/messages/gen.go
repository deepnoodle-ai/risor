package messages

//go:generate go run ../gen ../../../suite/messages.json catalog_gen.go messages
//go:generate go run ../gen ../../../suite/messages-queries.json queries_gen.go messages sourceQueriesJSON
//go:generate go run ../gen ../../../suite/messages-parse.json parse_gen.go messages sourceParseJSON
//go:generate go run ../gen ../../../suite/messages-check.json check_gen.go messages sourceCheckJSON
//go:generate go run ../gen ../../../suite/messages-entry-points.json entrypoints_gen.go messages sourceEntryPointsJSON
//go:generate go run ../gen ../../../suite/messages-recursion.json recursion_gen.go messages sourceRecursionJSON
//go:generate go run ../gen ../../../suite/messages-feedback.json feedback_gen.go messages sourceFeedbackJSON
