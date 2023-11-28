package dnsprovider

import (
	"os"
	"testing"

	"github.com/bizflycloud/external-dns-bizflycloud-webhook/cmd/webhook/init/configuration"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	config := configuration.Config{}
	_ = os.Setenv("BFC_APP_CREDENTIAL_ID", "e5d084f79fd5407da705f8df97332090")
	_ = os.Setenv("BFC_APP_CREDENTIAL_SECRET", "Fhkp66ClGHPFTnjHQId1RWrUoG14qMIK8IWT4GxURTNLfY2cAkVmpTwyJ-xYsDlEqrS0lqBSGG8DEGZma6OQgQ")

	dnsProvider, err := Init(config)
	assert.NotNil(t, dnsProvider)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	_ = os.Setenv("DOMAIN_FILTER", "vietquocxa.online")
	dnsProvider, err = Init(config)
	assert.NotNil(t, dnsProvider)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	_ = os.Unsetenv("BFC_APP_CREDENTIAL_ID")
	_ = os.Unsetenv("BFC_APP_CREDENTIAL_SECRET")
	_ = os.Unsetenv("DOMAIN_FILTER")
	_, err = Init(config)
	if err == nil {
		t.Errorf("expected to fail")
	}
}
