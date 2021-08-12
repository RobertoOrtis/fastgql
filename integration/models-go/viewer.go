package models

import "github.com/RobertoOrtis/fastgql/integration/remote_api"

type Viewer struct {
	User *remote_api.User
}
