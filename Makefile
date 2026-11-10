.PHONY: build web server run test dev-server dev-web clean

# Build the React app, then a single Go binary with the app embedded.
build: web server

web:
	cd web && npm install && npm run build
	@touch web/dist/.gitkeep

server:
	go build -o cinema ./cmd/cinema

run: build
	./cinema

test:
	go test ./...

# Development: run these in two terminals, then open http://localhost:5173
dev-server:
	go run ./cmd/cinema

dev-web:
	cd web && npm install && npm run dev

clean:
	rm -f cinema
	find web/dist -mindepth 1 ! -name .gitkeep -delete
