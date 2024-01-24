package mongo

import (
	"context"
	"testing"
	"time"

	"upload/pkg/file"
	"upload/shared/zombiekiller"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestKillZombie(t *testing.T) {
	const timeout = 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(func() { cancel() })

	uri := "mongodb://localhost:27017"
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Skipf("file: repo: mongo: failed to connect with test db: %v", err)
	}
	t.Cleanup(func() { client.Disconnect(ctx) })

	db := client.Database("Upload")
	coll := db.Collection("Files-kill_zombie")
	coll.Drop(ctx)
	t.Cleanup(func() { coll.Drop(ctx) })
	fileColl := NewRepo(coll)

	id, _ := uuid.Parse("6bd1edba-bfe6-4a18-b2f3-db1dc4043b32")
	f := file.File{ID: id, UploaderID: "0", CompanyID: "1", Name: "photo.png", Extension: ".png", ContentType: "image/png", Size: 256, StorageLocation: "https://storage.blob.com/" + id.String(), Description: "some random photo", SubmittedAt: time.Now()}
	fileColl.Insert(ctx, &f)

	cases := map[string]struct {
		in  uuid.UUID
		out error
	}{
		"basic kill zombie operation": {id, nil},
		"file does not exists":        {uuid.New(), zombiekiller.ErrNotFound},
	}

	for key, testcase := range cases {
		t.Run(key, func(t *testing.T) {
			if err := fileColl.KillZombie(testcase.in); err != testcase.out {
				t.Errorf("file: repo: mongo: test_kill_zombie: error mismatch (result = %v, expected = %v)\n", err, testcase.out)
			}
		})
	}
}
