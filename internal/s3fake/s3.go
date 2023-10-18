// Copyright 2023 The Kubernetes Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package s3fake

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	"github.com/johannesboyne/gofakes3"
)

// S3Fake is a fake S3 server.
type S3Fake struct {
	Address string
	Backend gofakes3.Backend
	Users   *sync.Map

	log logr.Logger
}

// NewS3Fake creates a new fake S3 server.
func NewS3Fake(log logr.Logger, address string, backend gofakes3.Backend) *S3Fake {
	return &S3Fake{
		log:     log,
		Address: address,
		Backend: backend,
		Users:   &sync.Map{},
	}
}

// Run starts the fake S3 server.
func (s *S3Fake) Run(ctx context.Context) error {
	faker := gofakes3.New(s.Backend)

	srv := http.Server{
		Addr:    s.Address,
		Handler: faker.Server(),
	}

	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			s.log.V(3).Error(err, "failed to shutdown server")
		}
	}()

	return srv.ListenAndServe()
}

// CreateBucket creates a bucket.
func (s *S3Fake) CreateBucket(bucket string) error {
	return s.Backend.CreateBucket(bucket)
}

// BucketExists checks if a bucket exists.
func (s *S3Fake) BucketExists(bucket string) (bool, error) {
	return s.Backend.BucketExists(bucket)
}

// DeleteBucket deletes a bucket.
func (s *S3Fake) DeleteBucket(bucket string) error {
	return s.Backend.DeleteBucket(bucket)
}

// CreateUser creates a user.
func (s *S3Fake) CreateUser(name string) (*User, error) {
	user := &User{
		Name: name,
	}

	// Generate a key pair for the user.
	user.genKeyPair()

	// Store the user in the map.
	s.Users.Store(name, user)

	return user, nil
}

// DeleteUser deletes a user.
func (s *S3Fake) DeleteUser(name string) error {
	s.Users.Delete(name)

	return nil
}

// UserExists checks if a user exists.
func (s *S3Fake) UserExists(name string) (*User, error) {
	i, ok := s.Users.Load(name)
	if !ok {
		return nil, nil
	}

	user, ok := i.(*User)
	if !ok {
		return nil, errors.New("failed to get user")
	}

	return user, nil
}
