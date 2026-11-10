package library

//go:generate go run ../gen ../../../suite/library.json table_gen.go library
//go:generate go run ../gen ../../../suite/library-groups.json groups_gen.go library sourceGroupsJSON
//go:generate go run ../gen ../../../suite/library-queries.json queries_gen.go library sourceQueriesJSON
