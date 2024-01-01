package jboss

import (
    "github.com/yhy0/Jie/pkg/protocols/httpx"
)

func CVE_2017_12149(url string, client *httpx.Client) bool {
    if req, err := client.Request(url+"/invoker/readonly", "GET", "", nil); err == nil {
        if req.StatusCode == 500 {
            return true
        }
    }
    return false
}
