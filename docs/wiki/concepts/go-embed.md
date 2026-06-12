# Go Embed

Go's `embed` feature (from Go 1.16+) is used in cc-connect to embed the compiled web dashboard into the Go binary via `embed.go`.

This allows the single binary to serve the web UI without external static files, simplifying deployment.

Cross-references: [tech-stack](../sources/tech-stack.md), [react](../entities/react.md)
