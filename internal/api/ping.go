package api

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/idna"

	"github.com/kite-plus/explore/internal/check"
	"github.com/kite-plus/explore/internal/policy"
)

// XML-RPC fault codes from the specification for fault code
// interoperability, which ping clients understand.
const (
	faultParse         = -32700
	faultMethod        = -32601
	faultParams        = -32602
	pingThanks         = "Thanks for the ping."
	methodPing         = "weblogUpdates.ping"
	methodExtendedPing = "weblogUpdates.extendedPing"
)

type pingRequest struct {
	URL string `json:"url"`
}

type xmlrpcCall struct {
	XMLName    xml.Name      `xml:"methodCall"`
	MethodName string        `xml:"methodName"`
	Params     []xmlrpcValue `xml:"params>param>value"`
}

// xmlrpcValue keeps only strings; a value without a type element is a
// string too.
type xmlrpcValue struct {
	String *string `xml:"string"`
	Text   string  `xml:",chardata"`
}

func (v xmlrpcValue) string() string {
	if v.String != nil {
		return strings.TrimSpace(*v.String)
	}
	return strings.TrimSpace(v.Text)
}

// ping brings a listed blog's next fetch forward; see docs/design/api.md
// section 6. It answers the same whether or not the address is listed.
func (s *Server) ping(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBody)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	if strings.Contains(c.ContentType(), "xml") || bytes.HasPrefix(bytes.TrimSpace(body), []byte("<")) {
		s.pingXMLRPC(c, body)
		return
	}
	var req pingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.fail(c, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	hosts, ok := pingHosts(req.URL)
	if !ok {
		s.fail(c, http.StatusBadRequest, codeInvalidURL)
		return
	}
	if !s.schedulePing(c, [][]string{hosts}) {
		return
	}
	c.Status(http.StatusAccepted)
}

// pingXMLRPC answers weblogUpdates.ping(name, url) and
// weblogUpdates.extendedPing(name, url, ...). WordPress sends the feed as
// the third parameter of extendedPing, the weblogs.com form as the fourth,
// so every later parameter that parses as an address is a fallback.
func (s *Server) pingXMLRPC(c *gin.Context, body []byte) {
	var call xmlrpcCall
	if err := xml.Unmarshal(body, &call); err != nil {
		xmlrpcFault(c, faultParse, "parse error: not well formed")
		return
	}
	method := strings.TrimSpace(call.MethodName)
	if method != methodPing && method != methodExtendedPing {
		xmlrpcFault(c, faultMethod, "requested method not found")
		return
	}
	if len(call.Params) < 2 {
		xmlrpcFault(c, faultParams, "expected the site name and address")
		return
	}
	site, ok := pingHosts(call.Params[1].string())
	if !ok {
		xmlrpcFault(c, faultParams, "the site address is not an http or https URL")
		return
	}
	candidates := [][]string{site}
	if method == methodExtendedPing {
		for _, p := range call.Params[2:] {
			if hosts, ok := pingHosts(p.string()); ok {
				candidates = append(candidates, hosts)
			}
		}
	}
	if !s.schedulePing(c, candidates) {
		return
	}
	c.Data(http.StatusOK, "text/xml; charset=utf-8", []byte(xml.Header+`<methodResponse><params><param><value><struct>`+
		`<member><name>flerror</name><value><boolean>0</boolean></value></member>`+
		`<member><name>message</name><value><string>`+pingThanks+`</string></value></member>`+
		`</struct></value></param></params></methodResponse>`+"\n"))
}

// schedulePing tries each set of hosts in turn until one names a listed
// blog. It reports false after answering with an error.
func (s *Server) schedulePing(c *gin.Context, candidates [][]string) bool {
	for _, hosts := range candidates {
		found, scheduled, err := s.Store.PingBlog(c.Request.Context(), hosts, policy.PingSpacing, policy.FetchLease)
		if err != nil {
			s.storeError(c, err)
			return false
		}
		if found {
			if scheduled {
				s.Log.Info("ping moved a fetch forward", "host", hosts[0])
			}
			break
		}
	}
	return true
}

// pingHosts is the host of an address, in the ASCII form blogs are listed
// under, with and without www.
func pingHosts(raw string) ([]string, bool) {
	u, err := check.ParseURL(raw)
	if err != nil {
		return nil, false
	}
	host, err := idna.Lookup.ToASCII(strings.TrimSuffix(u.Hostname(), "."))
	if err != nil || host == "" {
		return nil, false
	}
	host = strings.ToLower(host)
	if bare, ok := strings.CutPrefix(host, "www."); ok {
		return []string{host, bare}, true
	}
	return []string{host, "www." + host}, true
}

func xmlrpcFault(c *gin.Context, code int, message string) {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(message))
	c.Data(http.StatusOK, "text/xml; charset=utf-8", []byte(xml.Header+`<methodResponse><fault><value><struct>`+
		`<member><name>faultCode</name><value><int>`+strconv.Itoa(code)+`</int></value></member>`+
		`<member><name>faultString</name><value><string>`+escaped.String()+`</string></value></member>`+
		`</struct></value></fault></methodResponse>`+"\n"))
}
