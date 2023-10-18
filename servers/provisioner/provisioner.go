// Package provisioner
package provisioner

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/shanduur/cosi-driver-sample-s3-inmemory/internal/s3fake"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

// Server implements cosi.ProvisionerServer interface.
type Server struct {
	S3 *s3fake.S3Fake

	log logr.Logger
}

// Interface guards.
var _ cosi.ProvisionerServer = (*Server)(nil)

// New returns provisioner.Server with default values.
func New(logger logr.Logger, s3 *s3fake.S3Fake) *Server {
	return &Server{
		log: logger,
		S3:  s3,
	}
}

// DriverCreateBucket call is made to create the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
//  1. If a bucket that matches both name and parameters already exists, then OK (success) must be returned.
//  2. If a bucket by same name, but different parameters is provided, then the appropriate error code ALREADY_EXISTS must be returned.
func (s *Server) DriverCreateBucket(ctx context.Context, req *cosi.DriverCreateBucketRequest) (*cosi.DriverCreateBucketResponse, error) {
	if exists, err := s.S3.BucketExists(req.Name); err != nil {
		s.log.V(3).Error(err, "DriverCreateBucket: failed to check if bucket exists", "bucket", req.Name)
		return nil, status.Error(codes.Internal, err.Error())
	} else if !exists {
		s.log.V(3).Info("DriverCreateBucket: creating bucket", "bucket", req.Name)

		err := s.S3.CreateBucket(req.Name)
		if err != nil {
			s.log.V(3).Error(err, "DriverCreateBucket: failed to create bucket", "bucket", req.Name)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.log.V(3).Info("DriverCreateBucket: bucket created", "bucket", req.Name)
	return &cosi.DriverCreateBucketResponse{
		BucketId: req.Name,
		BucketInfo: &cosi.Protocol{
			Type: &cosi.Protocol_S3{
				S3: &cosi.S3{
					Region:           "eu-central-1",
					SignatureVersion: cosi.S3SignatureVersion_S3V2,
				},
			},
		},
	}, nil
}

// DriverDeleteBucket call is made to delete the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
// If the bucket has already been deleted, then no error should be returned.
func (s *Server) DriverDeleteBucket(ctx context.Context, req *cosi.DriverDeleteBucketRequest) (*cosi.DriverDeleteBucketResponse, error) {
	if exists, err := s.S3.BucketExists(req.BucketId); err != nil {
		s.log.V(3).Error(err, "DriverDeleteBucket: failed to check if bucket exists", "bucket", req.BucketId)
		return nil, status.Error(codes.Internal, err.Error())
	} else if exists {
		s.log.V(3).Info("DriverDeleteBucket: deleting bucket", "bucket", req.BucketId)

		err := s.S3.DeleteBucket(req.BucketId)
		if err != nil {
			s.log.V(3).Error(err, "DriverDeleteBucket: failed to delete bucket", "bucket", req.BucketId)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.log.V(3).Info("DriverDeleteBucket: bucket deleted", "bucket", req.BucketId)
	return &cosi.DriverDeleteBucketResponse{}, nil
}

// DriverGrantBucketAccess call grants access to an account.
// The account_name in the request shall be used as a unique identifier to create credentials.
//
// NOTE: this call needs to be idempotent.
// The account_id returned in the response will be used as the unique identifier for deleting this access when calling DriverRevokeBucketAccess.
// The returned secret does not need to be the same each call to achieve idempotency.
func (s *Server) DriverGrantBucketAccess(ctx context.Context, req *cosi.DriverGrantBucketAccessRequest) (*cosi.DriverGrantBucketAccessResponse, error) {
	user, err := s.S3.UserExists(req.Name)
	if err != nil {
		s.log.V(3).Error(err, "DriverGrantBucketAccess: failed to check if user exists", "user", req.Name)
		return nil, status.Error(codes.Internal, err.Error())
	} else if user == nil {
		s.log.V(3).Info("DriverGrantBucketAccess: creating user", "user", req.Name)

		user, err = s.S3.CreateUser(req.Name)
		if err != nil {
			s.log.V(3).Error(err, "DriverGrantBucketAccess: failed to create user", "user", req.Name)
			return nil, status.Error(codes.Internal, err.Error())
		}

		s.log.V(5).Info("DriverGrantBucketAccess: user created", "user", user)
	}

	s.log.V(3).Info("DriverGrantBucketAccess: access granted", "user", req.Name, "bucket", req.BucketId)
	return &cosi.DriverGrantBucketAccessResponse{
		AccountId: user.Name,
		Credentials: map[string]*cosi.CredentialDetails{
			"s3": &cosi.CredentialDetails{
				Secrets: map[string]string{
					"accessKeyID":     user.AccessKey,
					"accessSecretKey": user.SecretKey,
				},
			},
		},
	}, nil
}

// DriverRevokeBucketAccess call revokes all access to a particular bucket from a principal.
//
// NOTE: this call needs to be idempotent.
func (s *Server) DriverRevokeBucketAccess(ctx context.Context, req *cosi.DriverRevokeBucketAccessRequest) (*cosi.DriverRevokeBucketAccessResponse, error) {
	user, err := s.S3.UserExists(req.AccountId)
	if err != nil {
		s.log.V(3).Error(err, "DriverRevokeBucketAccess: failed to check if user exists", "user", req.AccountId)
		return nil, status.Error(codes.Internal, err.Error())
	} else if user != nil {
		s.log.V(3).Info("DriverRevokeBucketAccess: deleting user", "user", req.AccountId)

		if err := s.S3.DeleteUser(req.AccountId); err != nil {
			s.log.V(3).Error(err, "DriverRevokeBucketAccess: failed to delete user", "user", req.AccountId)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.log.V(3).Info("DriverRevokeBucketAccess: access revoked", "user", req.AccountId, "bucket", req.BucketId)
	return &cosi.DriverRevokeBucketAccessResponse{}, nil
}
