package routes

import "github.com/tedsuo/router"

const (
	CreateBuild  = "CreateBuild"
	GetBuild     = "GetBuild"
	UploadBits   = "UploadBits"
	DownloadBits = "DownloadBits"
	SetResult    = "SetResult"
	GetResult    = "GetResult"
	LogInput     = "LogInput"
	LogOutput    = "LogOutput"
)

var Routes = router.Routes{
	{Path: "/builds", Method: "POST", Handler: CreateBuild},
	{Path: "/builds", Method: "GET", Handler: GetBuild},

	{Path: "/builds/:guid/bits", Method: "POST", Handler: UploadBits},
	{Path: "/builds/:guid/bits", Method: "GET", Handler: DownloadBits},

	{Path: "/builds/:guid/result", Method: "PUT", Handler: SetResult},
	{Path: "/builds/:guid/result", Method: "GET", Handler: GetResult},

	{Path: "/builds/:guid/log/input", Method: "GET", Handler: LogInput},
	{Path: "/builds/:guid/log/output", Method: "GET", Handler: LogOutput},
}
