package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	vault "github.com/mittwald/vaultgo"
)

type VaultCreds struct {
	Data struct {
		User     string `json:"username"`
		Password string `json:"password"`
	} `json:"data"`
}

type DbConnection struct {
	Dbname   string
	Host     string
	Port     int
	User     string
	Password string
}

// Read (generate) credentials from our Vault server.
// Don't forget to update your Vault address and token.
func getDBConnectionConfig() DbConnection {
	c, err := vault.NewClient("http://vault.vault.svc:8200", vault.WithCaPath(""), vault.WithAuthToken("hvs.oFNGuqflt9gyXDy0WbIWbKUJ"))
	if err != nil {
		panic(err)
	}

	key := []string{"v1", "database", "creds", "dbuser"}
	options := &vault.RequestOptions{}
	response := &VaultCreds{}

	err = c.Read(key, response, options)
	if err != nil {
		panic(err)
	}

	return DbConnection{
		Dbname:   "postgres",
		Host:     "psql-postgres.psql.svc",
		Port:     5432,
		User:     response.Data.User,
		Password: response.Data.Password,
	}
}

// This function opens up a new Postgres connection to our server and returns it.
func openConnection() *sql.DB {
	config := getDBConnectionConfig()
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", config.Host, config.Port, config.User, config.Password, config.Dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Connected to PostgreSQL db using user <%s> and password <%s>\n", config.User, config.Password)
	return db
}

// Factory to create the function that handles the index request "/".
// It queries the database and return a join of all names in the users table. Pretty simple.
func newIndexHandler(db *sql.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`SELECT name FROM users`)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		names := []string{}
		for rows.Next() {
			var name string
			err = rows.Scan(&name)
			if err != nil {
				panic(err)
			}

			names = append(names, name)
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, strings.Join(names, ", "))
	}
}

func main() {
	// Setup DB connection
	db := openConnection()
	defer db.Close()

	// Setup router
	r := mux.NewRouter()
	r.HandleFunc("/", newIndexHandler(db))
	http.Handle("/", r)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)
	go func() {
		<-signals

		fmt.Println("Reloading: Terminating current connection and creating a new one.")
		db.Close()
		db = openConnection()
	}()

	// Start listening
	fmt.Println("Listening on 0.0.0.0:8002")
	http.ListenAndServe("0.0.0.1:8002", r)
}
