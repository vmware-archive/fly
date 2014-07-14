package routes

import "github.com/tedsuo/rata"

const (
	CreateBuild  = "CreateBuild"
	GetBuilds    = "GetBuilds"
	HijackBuild  = "HijackBuild"
	UploadBits   = "UploadBits"
	DownloadBits = "DownloadBits"
	SetResult    = "SetResult"
	GetResult    = "GetResult"
	LogInput     = "LogInput"
	LogOutput    = "LogOutput"
)

var Routes = rata.Routes{
	{Path: "/builds", Method: "POST", Name: CreateBuild},
	{Path: "/builds", Method: "GET", Name: GetBuilds},

	{Path: "/builds/:guid/bits", Method: "POST", Name: UploadBits},
	{Path: "/builds/:guid/bits", Method: "GET", Name: DownloadBits},

	{Path: "/builds/:guid/hijack", Method: "POST", Name: HijackBuild},

	{Path: "/builds/:guid/result", Method: "PUT", Name: SetResult},
	{Path: "/builds/:guid/result", Method: "GET", Name: GetResult},

	{Path: "/builds/:guid/log/input", Method: "GET", Name: LogInput},
	{Path: "/builds/:guid/log/output", Method: "GET", Name: LogOutput},
}
