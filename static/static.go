package static

import (
	"embed"
)

//go:embed index.html
var FS embed.FS

// func main() {
// 	tmpl, err := template.ParseFS(htmlFiles, "index.html")
// 	if err != nil {
// 		panic(err)
// 	}

// 	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
// 		data := struct {
// 			Title   string
// 			Message string
// 		}{
// 			Title:   "My Page",
// 			Message: "Hello, World!",
// 		}

// 		err := tmpl.Execute(w, data)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 		}
// 	})

// 	http.ListenAndServe(":8080", nil)
// }
