package bizflycloud

// Configuration holds configuration from environmental variables
type Configuration struct {
	APICredentialId     string `env:"BFC_APP_CREDENTIAL_ID,notEmpty"`
	APICredentialSecret string `env:"BFC_APP_CREDENTIAL_SECRET,notEmpty"`
	Debug               bool   `env:"IONOS_DEBUG" envDefault:"false"`
	DryRun              bool   `env:"DRY_RUN" envDefault:"false"`
	Region              string `env:"BFC_REGION" envDefault:"HN"`
	APIPageSize         int    `env:"BFC_API_PAGE_SIZE" envDefault:"100"`
}
