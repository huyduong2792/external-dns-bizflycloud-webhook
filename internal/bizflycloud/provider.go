package bizflycloud

import (
	"context"
	"fmt"

	"github.com/bizflycloud/external-dns-bizflycloud-webhook/pkg/endpoint"
	"github.com/bizflycloud/external-dns-bizflycloud-webhook/pkg/plan"
	"github.com/bizflycloud/external-dns-bizflycloud-webhook/pkg/provider"
	"github.com/bizflycloud/gobizfly"
	log "github.com/sirupsen/logrus"
)

const (
	// bizflyCloudCreate is a ChangeAction enum value
	bizflyCloudCreate = "CREATE"
	// bizflyCloudDelete is a ChangeAction enum value
	bizflyCloudDelete = "DELETE"
	// bizflyCloudUpdate is a ChangeAction enum value
	bizflyCloudUpdate = "UPDATE"
	// defaultBizflyCloudRecordTTL
	defaultBizflyCloudRecordTTL = 60
	// gobizfly auth method
	auth_method = "application_credential"
)

// // bizflyCloudDNS is the subset of the bizflycloud.DNSService that we actually use.  Add methods as required. Signatures must match exactly.
type bizflyCloudDNS interface {
	ListZones(ctx context.Context, opts *gobizfly.ListOptions) (*gobizfly.ListZoneResp, error)
	GetZone(ctx context.Context, zoneID string) (*gobizfly.ExtendedZone, error)
	CreateRecord(ctx context.Context, zoneID string, crpl interface{}) (*gobizfly.Record, error)
	UpdateRecord(ctx context.Context, recordID string, urpl interface{}) (*gobizfly.Record, error)
	DeleteRecord(ctx context.Context, recordID string) error
}

// BizflyCloudProvider is an implementation of Provider for BizflyCloud DNS.
type BizflyCloudProvider struct {
	provider.BaseProvider
	Client bizflyCloudDNS
	// only consider hosted zones managing domains ending in this suffix
	domainFilter endpoint.DomainFilter
	// page size when querying paginated APIs
	apiPageSize int
	DryRun      bool
}

type NormalRecord struct {
	Name string
	Type string
	TTL  int
	Data []string
}

// bizflyCloudChange differentiates between ChangActions
type bizflyCloudChange struct {
	Action       string
	NormalRecord NormalRecord
}

func SupportedRecordType(recordType string) bool {
	switch recordType {
	case "A", "AAAA", "CNAME", "SRV", "TXT":
		return true
	default:
		return false
	}
}

// getUpdateDNSRecordParam is a function that returns the appropriate Record Param based on the bizflyCloudChange passed in
func getUpdateDNSRecordParam(change bizflyCloudChange) gobizfly.UpdateNormalRecordPayload {
	return gobizfly.UpdateNormalRecordPayload{
		BaseUpdateRecordPayload: gobizfly.BaseUpdateRecordPayload{
			Name: change.NormalRecord.Name,
			TTL:  change.NormalRecord.TTL,
			Type: change.NormalRecord.Type,
		},
		Data: change.NormalRecord.Data,
	}
}

// getCreateDNSRecordParam is a function that returns the appropriate Record Param based on the bizflyCloudChange passed in
func getCreateDNSRecordParam(change bizflyCloudChange) gobizfly.CreateNormalRecordPayload {
	return gobizfly.CreateNormalRecordPayload{
		BaseCreateRecordPayload: gobizfly.BaseCreateRecordPayload{
			Name: change.NormalRecord.Name,
			TTL:  change.NormalRecord.TTL,
			Type: change.NormalRecord.Type,
		},
		Data: change.NormalRecord.Data,
	}
}

// NewBizflyCloudProvider initializes a new BizflyCloud DNS based Provider.
func NewBizflyCloudProvider(domainFilter endpoint.DomainFilter, config *Configuration) (provider.Provider, error) {
	fmt.Printf("%+v", config)
	client, err := gobizfly.NewClient(gobizfly.WithRegionName(config.Region))
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	token, err := client.Token.Create(
		ctx,
		&gobizfly.TokenCreateRequest{
			AuthMethod:    auth_method,
			AppCredID:     config.APICredentialId,
			AppCredSecret: config.APICredentialSecret})

	if err != nil {
		return nil, err
	}
	client.SetKeystoneToken(token)

	provider := &BizflyCloudProvider{
		Client:       client.DNS,
		domainFilter: domainFilter,
		apiPageSize:  config.APIPageSize,
		DryRun:       config.DryRun,
	}
	return provider, nil
}

// listDNSZonesWithAutoPagination performs automatic pagination of results on requests to bizflycloud.ListZones with custom limit values
func (p *BizflyCloudProvider) listDNSZonesWithAutoPagination(ctx context.Context) ([]gobizfly.Zone, error) {
	zones := []gobizfly.Zone{}
	listOptions := &gobizfly.ListOptions{Page: 1, Limit: p.apiPageSize}
	for {
		resp, err := p.Client.ListZones(ctx, listOptions)

		if err != nil {
			return nil, err
		}
		for _, zone := range resp.Zones {
			if p.domainFilter.Match(zone.Name) {
				zones = append(zones, zone)
			}
		}

		if listOptions.Page*listOptions.Limit >= resp.Meta.MaxResults {
			break
		}
		listOptions.Page++
	}
	return zones, nil
}

// Records returns the list of records.
func (p *BizflyCloudProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	zones, err := p.listDNSZonesWithAutoPagination(ctx)

	if err != nil {
		return nil, err
	}

	endpoints := []*endpoint.Endpoint{}
	for _, zone := range zones {
		detailZone, err := p.Client.GetZone(ctx, zone.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range detailZone.RecordsSet {
			if SupportedRecordType(r.Type) {
				name := r.Name + "." + zone.Name

				// root name is identified by @ and should be
				// translated to zone name for the endpoint entry.
				if r.Name == "@" {
					name = zone.Name
				}

				targets := make([]string, len(r.Data))
				for i, d := range r.Data {
					targets[i] = d.(string)
				}
				ep := endpoint.NewEndpointWithTTL(name, r.Type, endpoint.TTL(r.TTL), targets...)
				endpoints = append(endpoints, ep)
			}
		}
	}

	return endpoints, nil
}

// ApplyChanges applies a given set of changes in a given zone.
func (p *BizflyCloudProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {

	bizflycloudChanges := []*bizflyCloudChange{}

	for _, endpoint := range changes.Create {
		bizflycloudChanges = append(bizflycloudChanges, p.newBizflyCloudChange(bizflyCloudCreate, endpoint))
	}

	for _, endpoint := range changes.UpdateNew {
		bizflycloudChanges = append(bizflycloudChanges, p.newBizflyCloudChange(bizflyCloudUpdate, endpoint))
	}

	for _, endpoint := range changes.Delete {
		bizflycloudChanges = append(bizflycloudChanges, p.newBizflyCloudChange(bizflyCloudDelete, endpoint))
	}
	return p.submitChanges(ctx, bizflycloudChanges)
}

// submitChanges takes a zone and a collection of Changes and sends them as a single transaction.
func (p *BizflyCloudProvider) submitChanges(ctx context.Context, changes []*bizflyCloudChange) error {
	// return early if there is nothing to change
	if len(changes) == 0 {
		log.Info("All records are already up to date")
		return nil
	}

	zones, err := p.listDNSZonesWithAutoPagination(ctx)
	if err != nil {
		return err
	}
	// separate into per-zone change sets to be passed to the API.
	groupChangesByZoneID := p.groupChangesByZoneID(zones, changes)

	for zoneID, changes := range groupChangesByZoneID {
		detailZone, err := p.Client.GetZone(ctx, zoneID)
		if err != nil {
			return fmt.Errorf("could not fetch records from zone, %v", err)
		}
		for _, change := range changes {
			logFields := log.Fields{
				"record": change.NormalRecord.Name,
				"type":   change.NormalRecord.Type,
				"ttl":    change.NormalRecord.TTL,
				"action": change.Action,
				"zone":   zoneID,
			}

			log.WithFields(logFields).Info("Changing record...")

			if p.DryRun {
				continue
			}

			if change.Action == bizflyCloudUpdate {
				recordID := p.getRecordID(detailZone, change.NormalRecord)
				if recordID == "" {
					log.WithFields(logFields).Errorf("failed to find previous record: %v", change.NormalRecord)
					continue
				}
				recordParam := getUpdateDNSRecordParam(*change)

				_, err := p.Client.UpdateRecord(ctx, recordID, recordParam)
				if err != nil {
					log.WithFields(logFields).Errorf("failed to update record: %v", err)
				}
			} else if change.Action == bizflyCloudDelete {
				recordID := p.getRecordID(detailZone, change.NormalRecord)
				if recordID == "" {
					log.WithFields(logFields).Errorf("failed to find previous record: %v", change.NormalRecord)
					continue
				}
				err := p.Client.DeleteRecord(ctx, recordID)
				if err != nil {
					log.WithFields(logFields).Errorf("failed to delete record: %v", err)
				}
			} else if change.Action == bizflyCloudCreate {
				recordParam := getCreateDNSRecordParam(*change)
				_, err := p.Client.CreateRecord(ctx, zoneID, recordParam)
				if err != nil {
					log.WithFields(logFields).Errorf("failed to create record: %v", err)
				}
			}
		}
	}
	return nil
}

// groupChangesByZoneID separates a multi-zone change into a single change per zone.
func (p *BizflyCloudProvider) groupChangesByZoneID(zones []gobizfly.Zone, changeSet []*bizflyCloudChange) map[string][]*bizflyCloudChange {
	changes := make(map[string][]*bizflyCloudChange)
	zoneNameIDMapper := provider.ZoneIDName{}

	for _, z := range zones {
		zoneNameIDMapper.Add(z.ID, z.Name)
		changes[z.ID] = []*bizflyCloudChange{}
	}

	for _, c := range changeSet {
		zoneID, _ := zoneNameIDMapper.FindZone(c.NormalRecord.Name)
		if zoneID == "" {
			log.Debugf("Skipping record %s because no hosted zone matching record DNS Name was detected", c.NormalRecord.Name)
			continue
		}
		changes[zoneID] = append(changes[zoneID], c)
	}

	return changes
}

func (p *BizflyCloudProvider) getRecordID(zone *gobizfly.ExtendedZone, record NormalRecord) string {
	for _, zoneRecord := range zone.RecordsSet {
		name := zoneRecord.Name + "." + zone.Name
		if zoneRecord.Name == "@" {
			name = zone.Name
		}
		if name == record.Name && zoneRecord.Type == record.Type {
			return zoneRecord.ID
		}
	}
	return ""
}

func (p *BizflyCloudProvider) newBizflyCloudChange(action string, endpoint *endpoint.Endpoint) *bizflyCloudChange {
	ttl := defaultBizflyCloudRecordTTL

	if endpoint.RecordTTL.IsConfigured() {
		ttl = int(endpoint.RecordTTL)
	}

	return &bizflyCloudChange{
		Action: action,
		NormalRecord: NormalRecord{
			Name: endpoint.DNSName,
			TTL:  ttl,
			Type: endpoint.RecordType,
			Data: endpoint.Targets,
		},
	}
}
