package lailer

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/hr3lxphr6j/requests"
	"github.com/tidwall/gjson"

	"github.com/bililive-go/bililive-go/src/configs"
	"github.com/bililive-go/bililive-go/src/live"
	"github.com/bililive-go/bililive-go/src/live/internal"
	"github.com/bililive-go/bililive-go/src/pkg/utils"
)

const (
	domain = "m.lailer.net"
	cnName = "易直播"
)

func init() {
	live.Register(domain, new(builder))
}

type builder struct{}

func (b *builder) Build(u *url.URL) (live.Live, error) {
	return &Live{BaseLive: internal.NewBaseLive(u)}, nil
}

type Live struct {
	internal.BaseLive
}

// GetInfo: 优先调用 uservideolist 接口通过 living 字段判断开播状态；失败时回退页面解析。
func (l *Live) GetInfo() (*live.Info, error) {
	name := strings.Trim(strings.TrimPrefix(l.Url.Path, "/users/"), "/")
	if name == "" {
		name = strings.Trim(l.Url.Path, "/")
	}
	if name != "" {
		// 先用稳定接口获取用户信息（昵称与开播状态）
		userInfo := &url.URL{Scheme: "https", Host: domain, Path: "/h5/easylive/user/userInfo"}
		qUser := url.Values{}
		qUser.Set("name", name)
		qUser.Set("field", "all")
		userInfo.RawQuery = qUser.Encode()
		if resp, err := l.RequestSession.Get(userInfo.String(), live.CommonUserAgent,
			requests.Header("Accept", "application/json, text/plain, */*"),
			requests.Header("Referer", l.Url.String()),
		); err == nil && resp.StatusCode == http.StatusOK {
			if body, err2 := resp.Bytes(); err2 == nil {
				nick := gjson.GetBytes(body, "nickname").String()
				if nick != "" {
					living := gjson.GetBytes(body, "living").Bool()
					return &live.Info{
						Live:     l,
						HostName: nick,
						RoomName: "",
						Status:   living,
					}, nil
				}
			}
		}

		// 回退到 uservideolist 以获取状态/标题
		cfg := configs.GetCurrentConfig()
		sessionID := ""
		if cfg != nil {
			sessionID = cfg.Lailer.SessionID
		}
		api := &url.URL{Scheme: "https", Host: domain, Path: "/appgw/v2/uservideolist"}
		q := url.Values{}
		q.Set("name", name)
		q.Set("start", "0")
		if sessionID != "" {
			q.Set("sessionid", sessionID)
		}
		api.RawQuery = q.Encode()

		resp, err := l.RequestSession.Get(api.String(), live.CommonUserAgent,
			requests.Header("Custom-Agent", "gargantua v6.11.0 rv:20230803 Online (h5) Mozilla/5.0"),
			requests.Header("Referer", l.Url.String()),
			requests.Header("Accept", "application/json, text/plain, */*"),
		)
		if err == nil && resp.StatusCode == http.StatusOK {
			b, _ := resp.Bytes()
			if gjson.GetBytes(b, "retval").String() == "ok" {
				videosCount := gjson.GetBytes(b, "retinfo.videos.#").Int()
				if videosCount == 0 {
					return &live.Info{
						Live:     l,
						HostName: name,
						RoomName: "",
						Status:   false,
					}, nil
				}
				first := gjson.GetBytes(b, "retinfo.videos.0")
				living := first.Get("living").Int()
				return &live.Info{
					Live:     l,
					HostName: first.Get("nickname").String(),
					RoomName: first.Get("title").String(),
					Status:   living != 0,
				}, nil
			}
		}
	}

	// 回退：页面解析
	html, err := l.RequestSession.Get(l.Url.String(), live.CommonUserAgent)
	if err != nil {
		return nil, err
	}
	if html.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body, err := html.Text()
	if err != nil {
		return nil, err
	}

	var (
		strFilter = utils.NewStringFilterChain(utils.ParseUnicode, utils.UnescapeHTMLEntity)
		title     = strFilter.Do(utils.Match1(`(?i)<title[^>]*>([^<]+)<`, body))
		ogTitle   = strFilter.Do(utils.Match1(`(?i)property=['"]og:title['"][^>]*content=['"]([^'"]+)`, body))
		author    = strFilter.Do(utils.Match1(`(?i)property=['"]og:author['"][^>]*content=['"]([^'"]+)`, body))
		nick      = strFilter.Do(utils.Match1(`(?i)\"nick\"\s*:\s*\"([^\"]+)\"`, body))
	)

	hostName := author
	if hostName == "" {
		hostName = nick
	}
	roomName := ogTitle
	if roomName == "" {
		roomName = title
	}

	if roomName == "" {
		return nil, live.ErrInternalError
	}

	status := false
	lower := strings.ToLower(body)
	if strings.Contains(lower, ".m3u8") || strings.Contains(lower, ".flv") || strings.Contains(lower, "直播中") {
		status = true
	}

	return &live.Info{
		Live:     l,
		HostName: hostName,
		RoomName: roomName,
		Status:   status,
	}, nil
}

// GetStreamUrls: 先用页面回退解析到 m3u8/flv，后续可切到官方接口。
func (l *Live) GetStreamUrls() ([]*url.URL, error) {
	html, err := l.RequestSession.Get(l.Url.String(), live.CommonUserAgent)
	if err != nil {
		return nil, err
	}
	if html.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	body, err := html.Text()
	if err != nil {
		return nil, err
	}

	candidate := utils.Match1(`(?i)(https?:[^"'\s<>]+?\.(?:m3u8|flv)[^"'\s<>]*)`, body)
	if candidate == "" {
		return nil, live.ErrInternalError
	}
	u, err := url.Parse(candidate)
	if err != nil {
		return nil, err
	}
	return []*url.URL{u}, nil
}

func (l *Live) GetStreamInfos() (infos []*live.StreamUrlInfo, err error) {
	cfg := configs.GetCurrentConfig()
	sessionID := ""
	if cfg != nil {
		sessionID = cfg.Lailer.SessionID
	}
	name := strings.Trim(strings.TrimPrefix(l.Url.Path, "/users/"), "/")
	if name == "" {
		name = strings.Trim(l.Url.Path, "/")
	}
	// 先获取最新 vid
	apiList := &url.URL{Scheme: "https", Host: domain, Path: "/appgw/v2/uservideolist"}
	q := url.Values{}
	q.Set("name", name)
	q.Set("start", "0")
	if sessionID != "" {
		q.Set("sessionid", sessionID)
	}
	apiList.RawQuery = q.Encode()
	resp, err := l.RequestSession.Get(apiList.String(), live.CommonUserAgent,
		requests.Header("Custom-Agent", "gargantua v6.11.0 rv:20230803 Online (h5) Mozilla/5.0"),
		requests.Header("Referer", l.Url.String()),
		requests.Header("Accept", "application/json, text/plain, */*"),
	)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	b, _ := resp.Bytes()
	vid := gjson.GetBytes(b, "retinfo.videos.0.vid").String()
	if vid == "" {
		return nil, live.ErrInternalError
	}
	// 再根据 vid 获取真实播放地址
	apiWatch := &url.URL{Scheme: "https", Host: domain, Path: "/appgw/v2/watchstart"}
	q2 := url.Values{}
	q2.Set("vid", vid)
	if sessionID != "" {
		q2.Set("sessionid", sessionID)
	}
	apiWatch.RawQuery = q2.Encode()
	resp2, err := l.RequestSession.Get(apiWatch.String(), live.CommonUserAgent,
		requests.Header("Custom-Agent", "gargantua v6.11.0 rv:20230803 Online (h5) Mozilla/5.0"),
		requests.Header("Referer", "https://m.lailer.net/v/"+vid),
		requests.Header("Accept", "application/json, text/plain, */*"),
	)
	if err != nil {
		return nil, err
	}
	if resp2.StatusCode != http.StatusOK {
		return nil, live.ErrRoomNotExist
	}
	b2, _ := resp2.Bytes()
	if gjson.GetBytes(b2, "retval").String() != "ok" {
		return nil, live.ErrInternalError
	}
	playUrl := gjson.GetBytes(b2, "retinfo.play_url").String()
	if playUrl == "" {
		return nil, live.ErrInternalError
	}
	urls, err := utils.GenUrls(playUrl)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36",
		"Referer":    "https://m.lailer.net/v/" + vid,
	}
	infos = utils.GenUrlInfos(urls, headers)
	return
}

func (l *Live) GetPlatformCNName() string { return cnName }
