.PHONY: build run dev clean tailwind tailwind-watch

# Build the binary
build:
	go build -o podforge .

# Run the server
run: build
	./podforge

# Development mode: run with live reload (requires air)
dev:
	@command -v air >/dev/null 2>&1 || { echo "Install air: go install github.com/air-verse/air@latest"; exit 1; }
	air

# Build Tailwind CSS
tailwind:
	./tailwindcss -i static/css/input.css -o static/css/style.css --minify

# Watch Tailwind CSS changes
tailwind-watch:
	./tailwindcss -i static/css/input.css -o static/css/style.css --watch

# Clean build artifacts
clean:
	rm -f podforge
	rm -rf data/

# Cross-compile for all platforms
dist:
	GOOS=linux GOARCH=amd64 go build -o dist/podforge-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o dist/podforge-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o dist/podforge-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o dist/podforge-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o dist/podforge-windows-amd64.exe .
