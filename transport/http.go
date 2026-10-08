package transport

import (
	"io/fs"
	"log"
	"net/http"
)

// ServeClient starts the embedded HTTP server on the given port, serving
// the browser guitar co-pilot client from the provided filesystem.
// The clientFS should be an embed.FS or os.DirFS rooted at the client/ directory.
// The server runs in the background and does not block.
//
// Access the client at http://localhost:<port>/
func ServeClient(port string, clientFS fs.FS) {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(clientFS)))

	addr := ":" + port
	log.Printf("[HTTP] Guitar co-pilot client available at http://localhost%s", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("[HTTP] Server error: %v", err)
		}
	}()
}
