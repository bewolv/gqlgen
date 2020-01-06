package models

import "github.com/bewolv/gqlgen/integration/remote_api"

type Viewer struct {
	User *remote_api.User
}
