package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	srvFile "upload/pkg/file/httphandler"
	"upload/shared/blob"
	"upload/shared/composer"
	"upload/shared/zombiekiller"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args, os.Getenv, os.Getwd); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	getwd func() (string, error),
) error {
	// load environment variables
	if err := loadenv(args, getwd); err != nil {
		return err
	}

	// init mongodb (servers dependency)
	uri := getenv("MONGODB_URI")
	if uri == "" {
		return errors.New(`environment variable "MONGODB_URI" is either empty or does not exist`)
	}

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}

	defer func() {
		if err := mongoClient.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	dbName := getenv("MONGODB_NAME")
	if dbName == "" {
		return errors.New(`environment variable "MONGODB_NAME" is either empty or does not exist`)
	}
	dbClient := mongoClient.Database(dbName)

	// init zombie killer (servers dependency)
	doneChan := make(chan any)
	defer close(doneChan)
	const thrashBuffer = 1 << 10 * 1 // 1024 * (servers count)
	thrashChan := make(chan zombiekiller.KillOperation, thrashBuffer)
	go zombiekiller.ListenForKillOperations(doneChan, thrashChan)

	// init azure blob storage (servers dependency)
	blobstg, err := blob.NewAzureBlobStorage()
	if err != nil {
		return err
	}

	// compose servers into a single mux
	port := getenv("APP_PORT")
	if port == "" {
		return errors.New(`environment variable "APP_PORT" is either empty or does not exist`)
	}

	srv := composer.NewComposer()
	file := srvFile.NewServer(dbClient.Collection("files"), blobstg, thrashChan)
	if err := srv.Compose(file); err != nil {
		return err
	}

	log.Printf("Server listening on port %s...\n", port)
	log.Fatalln(http.ListenAndServe(":"+port, srv))

	return nil
}

func loadenv(args []string, getwd func() (string, error)) error {
	envflag := flag.String("env", "unset", "set which environment to load variables")
	flag.Parse()

	env := *envflag
	switch env {
	case "unset":
		fmt.Printf("Program executed without setting an environment. Using default option: %q.\n", "dev")
		fallthrough
	case "dev":
		fmt.Printf("Running Go app using %q environment.\n", "development")
		env = "dev.env"
	case "test":
		env = "test.env"
	default:
		return fmt.Errorf("invalid environment. valid options are: [%q, %q]", "dev", "test")
	}

	wd, err := getwd()
	if err != nil {
		return err
	}

	env = filepath.Join(wd, env)
	if err := godotenv.Overload(env); err != nil {
		return err
	}

	return nil
}
