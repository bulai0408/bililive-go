package lailer

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/hr3lxphr6j/bililive-go/src/live"
	"github.com/hr3lxphr6j/bililive-go/src/live/internal"
	"github.com/hr3lxphr6j/bililive-go/src/pkg/utils"
	"github.com/hr3lxphr6j/requests"
	"github.com/tidwall/gjson"
)

const (
	domain = "m.lailer.net"
	cnName = "易直播"
	vidUrl = "https://m.lailer.net/appgw/v2/uservideolist"
	userUrl =  "https://m.lailer.net/h5/easylive/user/userInfo"
	apiUrl = "https://m.lailer.net/appgw/v2/watchstart"
	sessionid = "nwydpo8UyxhrBPHoiFCInF0yi3NMM6lH"
)

func init() {
	live.Register(domain, new(builder))
}

type builder struct{}

func (b *builder) Build(url *url.URL, opt ...live.Option) (live.Live, error) {
	return &Live{
		BaseLive: internal.NewBaseLive(url, opt...),
	}, nil
}

type Live struct {
	internal.BaseLive
}

func (l *Live) requestRoomInfo() ([]byte, error) {


	userNumber := strings.Split(strings.Split(l.Url.Path, "/")[2], ".")[0]
	resp0, err0 := requests.Get(vidUrl, live.CommonUserAgent, requests.Query("name", userNumber),requests.Query("start", "0"),requests.Query("sessionid", sessionid))

	if err0 != nil {
		return nil, err0
	}
	if resp0.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body0, err0 := resp0.Bytes()
	if err0 != nil {
		return nil, err0
	}
	vid := gjson.GetBytes(body0,"retinfo.videos.0.vid")

	resp, err := requests.Get(apiUrl, live.CommonUserAgent, requests.Query("vid", vid.String()),requests.Query("sessionid", sessionid))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body, err := resp.Bytes()

	if err != nil {
		return nil, err
	}
	return body, nil
}

func (l *Live) GetInfo() (info *live.Info, err error) {
	userNumber := strings.Split(strings.Split(l.Url.Path, "/")[2], ".")[0]
	resp, err0 := requests.Get(userUrl, live.CommonUserAgent, requests.Header("el-auth", sessionid), requests.Query("name", userNumber),requests.Query("field", "all"))
	if err0 != nil {
		return nil, err0
	}
	if resp.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body, err0 := resp.Bytes()

	if err0 != nil {
		return nil, err0
	}

	info = &live.Info{
		Live:     l,
		HostName:  gjson.GetBytes(body,"nickname").String(),
		RoomName: gjson.GetBytes(body, "titleName").String(),
		Status:  gjson.GetBytes(body,"living").Bool(),
	}

	return info, nil
}

func (l *Live) GetStreamUrls() (us []*url.URL, err error) {
	resp, err := l.requestRoomInfo()

	if err != nil {
		return nil, err
	}

	u := gjson.GetBytes(resp, "retinfo.play_url").String()
	
	newU := strings.Replace(u, "tlive.jj17.cn", "tlive.lailer.net", -1)
	modifiedURL :=newU
	
	return utils.GenUrls(modifiedURL)
}

func (l *Live) GetPlatformCNName() string {
	return cnName
}
