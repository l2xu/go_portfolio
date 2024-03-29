// Package main implements a webserver that loads a zip file and stores the content in a mongoDB database. Furthermore it serves the content as a static website if needed.
package main

// import all needed packages
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

// The Page struct represents the data of each project in the portfolio
type Page struct {
	Title       string `bson:"title"`
	Short       string `bson:"short"`
	Image_url   string `bson:"image_url"`
	Description string `bson:"description"`
	Date        string `bson:"date"`
}

// The Pages struct represents the data of all projects in the portfolio
type Pages []Page

// The ps variable of type Pages holds all projects in the portfolio
var ps Pages

// The main function got executed first and loads the zip file, stores the content in the database (exports the website as static if needed) and starts the server
func main() {
	loadZip()
	loadContentFromDB()

	// Check for environment variable STATIC
	static, ok := os.LookupEnv("STATIC")
	if !ok {
		static = "false"
	}
	if static == "true" {
		staticExporter()
	}

	startServer()
}

// The loadZip function reads the zip file and extracts the content to the extracted folder and the static/img folder
func loadZip() {
	// reads the zip file
	r, err := zip.OpenReader("input/input.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()


	//Create the extracted folder if it does not exist
	if _, err := os.Stat("extracted"); os.IsNotExist(err) {
		os.Mkdir("extracted", 0755)
	}
	

	// Iterates through the files in the zip file
	for _, f := range r.File {
		// Open the file in the zip file
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		defer rc.Close()

		// Set the target path based on the file name
		var path string

		// Stores the projects.json file in the extracted folder and the images in the static/img folder and all other files in the extracted folder
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

// The loadContentFromDB function loads the content of the projects.json file into the database
func loadContentFromDB() {

	// Check for environment variable MONGO_URI
	mongoURI, ok := os.LookupEnv("MONGO_URI")
	if !ok {
		log.Fatal("MONGO_URI environment variable not set")
		// mongoURI = "mongodb://127.0.0.1:21017/"
	}

	// Sets the context for the connection to the database
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the database
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

	//create the databse and collection
	db := client.Database("go_portfolio")
	db.CreateCollection(context.Background(), "projects")

	// Get the collection
	myProjects := client.Database("go_portfolio").Collection("projects")

	// Read the JSON file that was extracted from the zip file
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

	//get all projects from the database 
	cursor, err := myProjects.Find(context.TODO(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}

	//iterate through all projects and append them to the ps variable
	for cursor.Next(context.TODO()) {
		var p Page
		err := cursor.Decode(&p)
		if err != nil {
			log.Fatal(err)
		}

		ps = append(ps, p)
	}
}

// The staticExporter function exports the website as static files and stores them in the out folder if the STATIC environment variable is set to true in the docker-compose file
func staticExporter() {

	//create the index page
	f, err := os.Create("./out/index.html")
	if err != nil {
		panic(err)
	}

	//render the index page
	renderPage(f, ps, "index.templ.html")

	//create a directory for the projects
	err = os.Mkdir("./out/projects", 0755)

	//go through all projects and create a page for each project
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

// The copyDir function copies the static folder to the out folder
// The function takes the source and destination folder as arguments in the form of strings
// The function returns an error if the source folder does not exist
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

// The startServer function starts the server and listens on port 9000
func startServer() {

	//serve the static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	//create the handlers for the index and project pages
	http.HandleFunc("/", makeIndexHandler())
	http.HandleFunc("/projects/", makeProjectHandler())

	//start the server
	log.Print("Listening on :9000 ....")
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

// The makeIndexHandler function creates the index page
func makeIndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//render the index page with the index.templ.html template
		err := renderPage(w, ps, "index.templ.html")
		if err != nil {
			log.Println(err)
		}
	}
}

// The makeProjectHandler function creates the project page
func makeProjectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		//get the url and split it to get the project name
		url := r.URL.Path
		url = url[10:]

		//load the project page according to the project name
		p, err := loadPage(url)
		if err != nil {
			log.Println(err)
		}

		//render the project page with the project.templ.html template
		err = renderPage(w, p, "project.templ.html")
		if err != nil {
			log.Println(err)
		}
	}
}

// The renderPage function renders the page with the given template and data
// The function takes the writer, the data and the template as arguments in the form of interfaces and strings
// The function returns an error if the template cannot be parsed or executed or if the template does not exist and returns nil if the template is successfully executed
func renderPage(w io.Writer, data interface{}, content string) error {

	//parse the template 
	temp, err := template.ParseFiles(
		filepath.Join("./templates", content),
	)
	if err != nil {
		return fmt.Errorf("renderPage.Parsefiles: %w", err)
	}

	//execute the template
	err = temp.ExecuteTemplate(w, content, data)
	if err != nil {
		return fmt.Errorf("renderPage.ExecuteTemplate: %w", err)
	}

	return nil
}

// The loadPage function loads the project page according to the project name
// The function takes the url as an argument in the form of a string
// The function returns the page and an error if the project name does not exist or returns the page and nil if the project name exists
func loadPage(url string) (Page, error) {

	//create a new page
	var p Page

	//search for the right project in the projects slice and assign the values to the new page (using the project name)
	for _, page := range ps {
		fmt.Print(page.Title)
		fullTitle := page.Title + ".html"

		if fullTitle == url {
			p.Title = page.Title
			p.Description = page.Description
			p.Image_url = page.Image_url
			p.Short = page.Short
			p.Date = page.Date
		}
	}
	return p, nil
}
