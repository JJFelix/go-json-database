package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jcelliott/lumber"
)

const Version = "1.0.0"

// logger
type (
	Logger interface {
		Fatal(string, ...interface{})
		Error(string, ...interface{})
		// Warning(string, ...interface{})
		Info(string, ...interface{})
		Debug(string, ...interface{})
		Trace(string, ...interface{})
	}

	Driver struct {
		mutex   sync.Mutex
		mutexes map[string]*sync.Mutex
		dir     string
		log     Logger
	}
)

type Options struct {
	Logger
}

// struct methods -> (d *Driver)
// initialize the db
func New(dir string, options *Options) (*Driver, error) {
	dir = filepath.Clean(dir)

	opts := Options{}
	if options != nil {
		opts = *options
	}

	if opts.Logger == nil {
		opts.Logger = lumber.NewConsoleLogger((lumber.INFO))
	}

	driver := Driver{
		dir:     dir,
		mutexes: make(map[string]*sync.Mutex),
		log:     opts.Logger,
	}

	if _, err := os.Stat(dir); err != nil {
		opts.Logger.Debug("Using '%s' (database already exists)\n", dir)
		return &driver, nil
	}

	opts.Logger.Debug("Creating the database at '%s'...\n ", dir)
	return &driver, os.MkdirAll(dir, 0755)
}

// write data to db
func (d *Driver) Write(collection, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("missing collections - no place to save record")
	}
	if resource == "" {
		return fmt.Errorf("missing resource - unable to save record (no name)")
	}

	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, collection)
	finalPath := filepath.Join(dir, resource+".json")
	tempPath := finalPath + ".tmp"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}
	b = append(b, byte('\n'))
	if err := os.WriteFile(tempPath, b, 0644); err != nil {
		return err
	}

	return os.Rename(tempPath, finalPath)
}

// Read data from db
func (d *Driver) Read(collection string, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("missing collection - unable to read record")
	}

	if resource == "" {
		return fmt.Errorf("missing resource - unable to read record(no name)")
	}

	record := filepath.Join(d.dir, collection, resource)
	if _, err := stat(record); err != nil {
		return err
	}

	b, err := os.ReadFile(record + ".json")
	if err != nil {
		return err
	}

	return json.Unmarshal(b, &v)
}

// Read all data from db
func (d *Driver) ReadAll(collection string) ([]string, error) {
	if collection == "" {
		return nil, fmt.Errorf("missing collection - unable to read record")
	}

	dir := filepath.Join(d.dir, collection)
	if _, err := stat(dir); err != nil{
		return nil, err
	}

	files, _ := os.ReadDir(dir)

	var records []string

	for _, file := range files{
		b, err := os.ReadFile(filepath.Join(dir, file.Name()))	
		if err != nil {
			return nil, err
		}

		records = append(records, string(b))
	}
	return records, nil
}

// Delete data from db
func (d *Driver) Delete(collection, resource string) error {
	path := filepath.Join(collection, resource)
	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, path)
	switch fi, err := stat(dir);{
	case fi == nil, err != nil:
		return fmt.Errorf("unable to find file or directory named %v\n", path)
	case fi.Mode().IsDir():
		return os.RemoveAll(dir)
	case fi.Mode().IsRegular():
		return os.RemoveAll(dir + ".json")
	}

	return nil

}

func (d *Driver) getOrCreateMutex(collection string) *sync.Mutex {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	m, ok := d.mutexes[collection]
	if !ok {
		m = &sync.Mutex{}
		d.mutexes[collection] = m
	}

	return m
}

func stat(path string) (fi os.FileInfo, err error) {
	if fi, err = os.Stat(path); os.IsNotExist(err) {
		fi, err = os.Stat(path + ".json")
	}
	return
}

type Address struct {
	City    string
	State   string
	Country string
	Pincode json.Number
}

type User struct {
	Name    string
	Age     json.Number
	Contact string
	Company string
	Address Address
}

func main() {
	dir := "./" // where files will reside

	db, err := New(dir, nil)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	// Hard-coding values into the db
	// you can create an api to send the data directly

	employees := []User{
		{"John", "23", "+254701028374", "IFAware Technologies", Address{"Nairobi City", "Nairobi", "Kenya", "00100"}},
		{"James", "25", "+1741628374", "Google", Address{"San Francisco", "California", "USA", "20409"}},
		{"Pedro", "22", "+1771828374", "Microsoft", Address{"Palo Alto", "California", "USA", "43693"}},
		{"Cole", "21", "+54751088374", "Amazon", Address{"Lisbon City", "Lisbon", "Portugal", "39100"}},
		{"Malo", "20", "+67706028974", "OpenAI", Address{"Oslo", "Greater Oslo", "Sweden", "94630"}},
		{"Nico", "22", "+18702028376", "Netflix", Address{"Moscow", "West Russia", "Russia", "42321"}},
	}

	// write into db
	for _, value := range employees {
		db.Write("users", value.Name, User{
			Name:    value.Name,
			Age:     value.Age,
			Contact: value.Contact,
			Company: value.Company,
			Address: value.Address,
		})
	}

	// Read DB function
	records, err := db.ReadAll("users")
	if err != nil {
		fmt.Println("Error: ", err)
	}
	fmt.Println(records) // records are in json format

	allusers := []User{}

	// unmarshal from json to go-understandable
	for _, f := range records {
		employeeFound := User{}
		if err := json.Unmarshal([]byte(f), &employeeFound); err != nil {
			fmt.Println("Error:", err)
		}
		allusers = append(allusers, employeeFound)
	}
	fmt.Println(allusers)

	// db delete
	if err := db.Delete("users", "Malo"); err != nil{
		fmt.Println("Error:", err)
	}

	// if err := db.Delete("users", ""); err != nil{
	// 	fmt.Println("Error:", err)
	// }

}
