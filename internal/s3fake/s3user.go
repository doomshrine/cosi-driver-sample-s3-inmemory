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
	"math/rand"
)

var custset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/+=")

type User struct {
	// ID is the ID of the user.
	ID string

	// Name is the name of the user.
	Name string

	// accessKey is the access key for the user.
	AccessKey string

	// secretKey is the secret key for the user.
	SecretKey string
}

// genKeyPair generates a key pair for the user. It will replace any existing key pair.
func (u *User) genKeyPair() {
	u.AccessKey = genKey(20)
	u.SecretKey = genKey(40)
}

func genKey(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = custset[rand.Intn(len(custset))]
	}
	return string(b)
}
