package main

import (
	"context"
	"os"

	// "encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"

	// "io/ioutil"
	"log"
	"net/http"

	"path/filepath"
	"time"

	// "github.com/russross/blackfriday/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	srcDir = flag.String("src", "./projects", "Inhalte -Dir.")
	tmpDir = flag.String("tmp", "./templates", "Template -Dir.")
)

// All the data that is needed for the template
type Page struct {
	Title       string `bson:"title"`
	Short       string `bson:"short"`
	Image_url   string `bson:"image_url"`
	Description string `bson:"description"`
	Date        string `bson:"date"`
}

type Pages []Page

var ps Pages

// Load the content from the mongodb and stores it in ps slice
func loadContent() {

	//CHECK FOR ENV VARIABLE
	// mongoURI, ok := os.LookupEnv("MONGO_URI")
	// if !ok {
	// 	log.Fatal("MONGO_URI environment variable not set")
	// }

	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()
	//replace with mongoUIR string
	opt := options.Client().ApplyURI("mongodb://127.0.0.1:21017/")
	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		log.Fatal(err)
	}
	// check for connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal(err)
	}

	myProjects := client.Database("go_portfolio").Collection("projects")

	// // Find all projects
	cursor, err := myProjects.Find(context.TODO(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}

	for cursor.Next(context.TODO()) {

		var p Page
		err := cursor.Decode(&p)
		if err != nil {
			log.Fatal(err)
		}

		ps = append(ps, p)

	}

}

// Exports all sites to the ./out directory
func staticExporter() {

	//create the index
	f, err := os.Create("./out/index.html")
	if err != nil {
		panic(err)
	}

	renderPage(f, ps, "index.templ.html")

	//create all other pages
	//go through all projects
	for _, p := range ps {
		filename := fmt.Sprintf("./out/%s.html", p.Title)
		f, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		renderPage(f, p, "project.templ.html")

	}

	//copy the static folder
	err = copyDir("./static/", "out/static/")
	if err != nil {
		panic(err)
	}

}

// Start the server and handle the requests
// Sets the directory for static files
func main() {

	loadContent()
	staticExporter()

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

		err := renderPage(w, ps, "index.templ.html")
		if err != nil {
			log.Println(err)
		}

	}
}

// Here the project page is created with the project.templ.html
func makePageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		//get the url and split it to get the project name
		url := r.URL.Path
		url = url[10:]

		p, err := loadPage(url)

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
		filepath.Join(*tmpDir, content),
	)

	if err != nil {
		return fmt.Errorf("renderPage.Parsefiles: %w", err)
	}
	err = temp.ExecuteTemplate(w, content, data)

	if err != nil {
		return fmt.Errorf("renderPage.ExecuteTemplate: %w", err)
	}

	return nil
}

// here the struct get filled with data
func loadPage(url string) (Page, error) {
	var p Page

	//search for the project in the struct
	for _, page := range ps {
		if page.Title == url {
			p.Title = page.Title
			p.Description = page.Description
			p.Image_url = page.Image_url
			p.Short = page.Short
			p.Date = page.Date

		}
	}

	return p, nil
}

// This function copies the static folder
func copyDir(src string, dst string) error {
	// Create the destination directory
	err := os.MkdirAll(dst, 0755)
	if err != nil {
		return err
	}

	// Walk through the source directory
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Compute the relative path of the file
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Compute the destination path
		dstPath := filepath.Join(dst, rel)

		// Check if the file is a directory
		if info.IsDir() {
			// Create the directory
			return os.MkdirAll(dstPath, info.Mode())
		} else {
			// Open the source file
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			// Create the destination file
			dstFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			// Copy the file
			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}

			// Preserve the file's mode
			return os.Chmod(dstPath, info.Mode())
		}
	})
}
