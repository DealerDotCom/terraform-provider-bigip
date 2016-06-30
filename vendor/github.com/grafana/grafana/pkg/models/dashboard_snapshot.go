package models

import (
	"time"

	"github.com/grafana/grafana/pkg/components/simplejson"
)

// DashboardSnapshot model
type DashboardSnapshot struct {
	Id          int64
	Name        string
	Key         string
	DeleteKey   string
	OrgId       int64
	UserId      int64
	External    bool
	ExternalUrl string

	Expires time.Time
	Created time.Time
	Updated time.Time

	Dashboard *simplejson.Json
}

// DashboardSnapshotDTO without dashboard map
type DashboardSnapshotDTO struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	DeleteKey   string `json:"deleteKey"`
	OrgId       int64  `json:"orgId"`
	UserId      int64  `json:"userId"`
	External    bool   `json:"external"`
	ExternalUrl string `json:"externalUrl"`

	Expires time.Time `json:"expires"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// -----------------
// COMMANDS

type CreateDashboardSnapshotCommand struct {
	Dashboard *simplejson.Json `json:"dashboard" binding:"Required"`
	Name      string           `json:"name"`
	Expires   int64            `json:"expires"`

	// these are passed when storing an external snapshot ref
	External  bool   `json:"external"`
	Key       string `json:"key"`
	DeleteKey string `json:"deleteKey"`

	OrgId  int64 `json:"-"`
	UserId int64 `json:"-"`

	Result *DashboardSnapshot
}

type DeleteDashboardSnapshotCommand struct {
	DeleteKey string `json:"-"`
}

type GetDashboardSnapshotQuery struct {
	Key string

	Result *DashboardSnapshot
}

type DashboardSnapshots []*DashboardSnapshot

type GetDashboardSnapshotsQuery struct {
	Name  string
	Limit int
	OrgId int64

	Result DashboardSnapshots
}
