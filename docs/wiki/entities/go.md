# Go

Go is the primary backend language for cc-connect, version 1.25.0 as specified in `go.mod`.

The entire backend — core engine, platform adapters, agent adapters, CLI, and daemon — is written in Go. The module path is `github.com/chenhg5/cc-connect`.

Go was chosen for its concurrency model (goroutines, channels), single-binary compilation, and cross-platform support, all of which are critical for a bridge service managing many simultaneous messaging sessions.

Cross-references: [tech-stack](../sources/tech-stack.md), [plugin-architecture](./plugin-architecture.md)
