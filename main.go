package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/russross/blackfriday/v2"
)

var (
	srcDir = flag.String("src", "./projects", "Inhalte -Dir.")
	tmpDir = flag.String("tmp", "./templates", "Template -Dir.")
)

// All the data that is needed for the template
type Page struct {
	Title   string
	Content template.HTML
	Image   string
	Link    string
}

type Pages []Page

// Start the server and handle the requests
// Sets the directory for static files
func main() {

	flag.Parse()
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", makeIndexHandler())
	http.HandleFunc("/projects/", makePageHandler())

	//start the server
	log.Print("Listening on :9000 ....")
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatal(err)
	}

}

// Here the index page is created with the index.templ.html
func makeIndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ps, err := loadPages(*srcDir)
		if err != nil {
			log.Println(err)
		}
		err = renderPage(w, ps, "index.templ.html")
		if err != nil {
			log.Println(err)
		}
	}
}

// Here the project page is created with the project.templ.html
func makePageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := r.URL.Path[len("/projects/"):]
		fpath := filepath.Join(*srcDir, f)
		p, err := loadPage(fpath)
		if err != nil {
			log.Println(err)
		}
		err = renderPage(w, p, "project.templ.html")
		if err != nil {
			log.Println(err)
		}
	}
}

// Here the template page is rendered
func renderPage(w io.Writer, data interface{}, content string) error {
	temp, err := template.ParseFiles(
		filepath.Join(*tmpDir, "base.templ.html"),
		filepath.Join(*tmpDir, content),
	)
	if err != nil {
		return fmt.Errorf("renderPage.Parsefiles: %w", err)
	}
	err = temp.ExecuteTemplate(w, "base", data)
	if err != nil {
		return fmt.Errorf("renderPage.ExecuteTemplate: %w", err)
	}
	return nil
}

// here the struct get filled with data
func loadPage(fpath string) (Page, error) {
	var p Page
	fi, err := os.Stat(fpath)
	if err != nil {
		return p, fmt.Errorf("loadPage: %w", err)
	}
	p.Title = fi.Name()
	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return p, fmt.Errorf("loadPage.ReadFile: %w", err)
	}
	p.Content = template.HTML(blackfriday.Run(b))
	return p, nil
}

// here all apges are loaded and returned so it can be used in the index page
func loadPages(src string) (Pages, error) {
	var ps Pages
	fs, err := ioutil.ReadDir(src)
	if err != nil {
		return ps, fmt.Errorf("loadPages.ReadDir: %w", err)
	}
	for _, f := range fs {
		if f.IsDir() {
			continue
		}
		fpath := filepath.Join(src, f.Name())
		p, err := loadPage(fpath)
		if err != nil {
			return ps, fmt.Errorf("loadPages.loadPage: %w", err)
		}
		ps = append(ps, p)
	}
	return ps, nil
}
