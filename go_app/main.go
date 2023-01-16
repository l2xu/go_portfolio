package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
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

func main() {
	loadZip()
	loadContentFromDB()

	// CHECK FOR ENV VARIABLE
	static, ok := os.LookupEnv("STATIC")
	if !ok {
		static = "false"
	}
	if static == "true" {
		staticExporter()
	}

	startServer()
}

func loadZip() {

	r, err := zip.OpenReader("input/input.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	// Iterieren Sie durch die Dateien in der ZIP-Datei
	for _, f := range r.File {
		// Öffnen Sie jede Datei
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		defer rc.Close()

		// Set the target path based on the file name
		var path string
		if f.Name == "projects.json" {
			path = filepath.Join("extracted", f.Name)
		} else if filepath.Dir(f.Name) == "images" {
			path = filepath.Join("static", "img", filepath.Base(f.Name))
		} else {
			path = filepath.Join("extracted", f.Name)
		}

		// Create the target directory, if it does not exist
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			// Create the file
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			// Write the contents of the opened file to the target
			_, err = io.Copy(f, rc)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func loadContentFromDB() {

	// CHECK FOR ENV VARIABLE
	mongoURI, ok := os.LookupEnv("MONGO_URI")
	if !ok {
		// log.Fatal("MONGO_URI environment variable not set")
		mongoURI = "mongodb://127.0.0.1:21017/"
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()
	//replace with mongoUIR string
	//"mongodb://127.0.0.1:21017/"
	opt := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		log.Fatal(err)
	}
	// check for connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal(err)
	}

	//drop databse
	client.Database("go_portfolio").Drop(context.Background())

	//create the databse and collection if not exist
	db := client.Database("go_portfolio")
	db.CreateCollection(context.Background(), "projects")

	myProjects := client.Database("go_portfolio").Collection("projects")

	// Read the JSON file
	data, err := ioutil.ReadFile("extracted/projects.json")
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshal the JSON into a struct
	var dataList []Page
	err = json.Unmarshal(data, &dataList)
	if err != nil {
		log.Fatal(err)
	}

	// Insert the struct into the MongoDB collection
	for _, d := range dataList {
		_, err := myProjects.InsertOne(context.TODO(), d)
		if err != nil {
			log.Fatal(err)
		}
	}

	//get all projects
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

func staticExporter() {

	//create the index
	f, err := os.Create("./out/index.html")
	if err != nil {
		panic(err)
	}

	renderPage(f, ps, "index.templ.html")

	//create a directory for the projects
	err = os.Mkdir("./out/projects", 0755)

	//create all other pages
	//go through all projects
	for _, p := range ps {
		filename := fmt.Sprintf("./out/projects/%s.html", p.Title)
		f, err := os.Create(filename)
		if err != nil {
			panic(err)
		}
		renderPage(f, p, "project.templ.html")

	}

	//copy the static folder
	err = copyDir("./static/", "./out/static/")
	if err != nil {
		panic(err)
	}
}
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

func startServer() {

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", makeIndexHandler())
	http.HandleFunc("/projects/", makeProjectHandler())

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
func makeProjectHandler() http.HandlerFunc {
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
		filepath.Join("./templates", content),
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
		fmt.Print(page.Title)
		peter := page.Title + ".html"

		if peter == url {
			p.Title = page.Title
			p.Description = page.Description
			p.Image_url = page.Image_url
			p.Short = page.Short
			p.Date = page.Date
		}
	}
	return p, nil
}

