// this is a sample echo rest api module
package user

import (
	"fmt"
	"strings"
	"sync"
)

var (
	users = users_{
		id: 1,
		data: map[string]*User{
			"tom": &User{
				Id:    1,
				Name:  "tom",
				Phone: "999",
			},
		},
	}
)

type users_ struct {
	sync.RWMutex
	id   int
	data map[string]*User
}

func createUser(in *CreateUserInput) (int, error) {
	users.Lock()
	defer users.Unlock()

	users.id++
	users.data[in.Name] = &User{
		Id:    users.id,
		Name:  in.Name,
		Phone: in.Phone,
	}

	return users.id, nil
}

func createUsers(in []CreateUserInput) (int, error) {
	for _, v := range in {
		createUser(&v)
	}
	return len(in), nil
}

func getUsers(in *GetUsersInput) (total int, list []*User, err error) {
	if in.Query == nil {
		for _, v := range users.data {
			list = append(list, v)
		}
	} else {
		for k, v := range users.data {
			if strings.Contains(k, *in.Query) {
				list = append(list, v)
			}
		}
	}

	total = len(list)
	return
}

func getUser(userName string) (*User, error) {
	users.RLock()
	defer users.RUnlock()
	if ret, ok := users.data[userName]; ok {
		return ret, nil
	}

	return nil, fmt.Errorf("user %s not found ", userName)
}

func updateUser(in *UpdateUserInput) (*User, error) {
	users.Lock()
	defer users.Unlock()
	if ret, ok := users.data[in.Name]; ok {
		ret.Phone = in.Phone
		return ret, nil
	}
	return nil, fmt.Errorf("user %s not found ", in.Name)
}

func deleteUser(userName string) (*User, error) {
	users.Lock()
	defer users.Unlock()
	if ret, ok := users.data[userName]; ok {
		delete(users.data, userName)
		return ret, nil
	}
	return nil, fmt.Errorf("user %s not found ", userName)

}
