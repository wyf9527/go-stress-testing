/**
* Created by GoLand.
* User: link1st
* Date: 2019-08-15
* Time: 21:03
 */

package client

import (
	"go-stress-testing/heper"
	"io"
	"net/http"
	"time"
)

// HTTP 请求
// method 方法 GET POST
// url 请求的url
// body 请求的body
// headers 请求头信息
// timeout 请求超时时间
func HttpRequest(client *http.Client, method, url string, body io.Reader, headers map[string]string, timeout time.Duration) (resp *http.Response, requestTime uint64, err error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {

		return
	}
	// 在req中设置Host，解决在header中设置Host不生效问题
	if _, ok := headers["Host"]; ok {
		req.Host = headers["Host"]
	}
	// 设置默认为utf-8编码
	if _, ok := headers["Content-Type"]; !ok {
		if headers == nil {
			headers = make(map[string]string)
		}
		headers["Content-Type"] = "application/x-www-form-urlencoded; charset=utf-8"
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	startTime := time.Now()
	resp, err = client.Do(req)
	requestTime = uint64(heper.DiffNano(startTime))
	if err != nil {
		return
	}

	// bytes, err := json.Marshal(req)
	// fmt.Printf("%#v \n", req)

	return
}
