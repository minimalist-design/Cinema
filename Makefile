.PHONY: build web server run test dev-server dev-web clean dist

# Build the React app, then a single Go binary with the app embedded.
build: web server

web:
	cd web && npm install && npm run build

# The Go code embeds web/dist (see web/embed.go), which must exist and hold
# at least one file. Until the React app is built, a placeholder stands in
# and the server answers "frontend not built". Vite empties the folder when
# it builds, removing the placeholder.
dist:
	@mkdir -p web/dist
	@[ -n "$$(ls -A web/dist)" ] || touch web/dist/.placeholder

server: dist
	go build -o cinema ./cmd/cinema

run: build
	./cinema

test: dist
	go test ./...

# Development: run these in two terminals, then open http://localhost:5173
dev-server: dist
	go run ./cmd/cinema

dev-web:
	cd web && npm install && npm run dev

clean:
	rm -f cinema
	rm -rf web/dist
