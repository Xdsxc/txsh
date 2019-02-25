package twilio

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xdsxc/txsh/internal/config"
)

type APIError struct {
	Code     int
	Message  string
	MoreInfo string `json:"more_info"`
	Status   int
}

func (a *APIError) Error() string {
	return fmt.Sprintf("SMSSender: received code %d from twilio: %s", a.Code, a.Message)
}

type SMSSender struct {
	url string
	cfg config.Config
}

func NewSMSSender(c config.Config) SMSSender {
	return SMSSender{
		url: fmt.Sprintf("%s/Accounts/%s/Messages.json", apiRoot, c.Twilio.Sender.AccountSID),
		cfg: c,
	}
}

func (s *SMSSender) Send(to, body string) error {
	v := url.Values{}
	v.Set("To", to)
	v.Set("From", s.cfg.Twilio.Sender.PhoneNumber)
	v.Set("Body", body)

	req, err := http.NewRequest("POST", s.url, strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(s.cfg.Twilio.Sender.AccountSID, s.cfg.Twilio.Sender.AuthToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		retErr := &APIError{}
		if err := json.NewDecoder(resp.Body).Decode(&retErr); err != nil {
			return fmt.Errorf("SMSSender: received code %d from twilio: %s", resp.StatusCode, body)
		}

		return retErr
	}

	return nil
}

type SMSHandler struct {
	CB func(id, body string) string

	logger logrus.FieldLogger
}

func NewSMSHandler(CB func(string, string) string) SMSHandler {
	return SMSHandler{
		CB: CB,

		logger: logrus.WithField("component", "twilio.SMSHandler"),
	}

}

func (s *SMSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type message struct {
		Content string `xml:",chardata"`
	}

	type twilioSMSResponse struct {
		XMLName xml.Name `xml:"Response"`
		Message message
	}

	if err := r.ParseForm(); err != nil {
		s.logger.WithError(err).Errorf("error parsing form")
	}

	id := r.Form.Get("From")
	if id == "" {
		s.logger.Errorf("empty ID")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	body := r.Form.Get("Body")
	if body == "" {
		s.logger.Errorf("empty body")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	responseContent := s.CB(id, body)
	resp := twilioSMSResponse{
		Message: message{
			Content: responseContent,
		},
	}

	w.Header().Set("Content-Type", "text/xml")
	if err := xml.NewEncoder(w).Encode(resp); err != nil {
		s.logger.WithError(err).Errorf("error encoding response")
		http.Error(w, "error encoding response", http.StatusInternalServerError)
		return
	}
}
