package bizflycloud

import (
	"context"
	"errors"
	"testing"

	"github.com/bizflycloud/external-dns-bizflycloud-webhook/pkg/endpoint"
	"github.com/bizflycloud/external-dns-bizflycloud-webhook/pkg/plan"
	"github.com/bizflycloud/gobizfly"
	"github.com/maxatome/go-testdeep/td"
	"github.com/stretchr/testify/assert"
)

type MockAction struct {
	Name       string
	ZoneId     string
	RecordData gobizfly.Record
}

type mockBizflyCloudClient struct {
	Zones   map[string]string
	Records map[string]gobizfly.Record
	Actions []MockAction
}

var ExampleRecrods = []gobizfly.Record{
	{
		ID:     "R001",
		ZoneID: "Z001",
		Name:   "foobar",
		Type:   endpoint.RecordTypeA,
		TTL:    120,
		Data:   makeRecordData([]string{"1.2.3.4", "3.4.5.6"}),
	},
	{
		ID:     "R002",
		ZoneID: "Z001",
		Name:   "foo",
		Type:   endpoint.RecordTypeA,
		TTL:    120,
		Data:   makeRecordData([]string{"3.4.5.6"}),
	},
	{
		ID:     "R003",
		ZoneID: "Z002",
		Name:   "bar",
		Type:   endpoint.RecordTypeA,
		TTL:    1,
		Data:   makeRecordData([]string{"2.3.4.5"}),
	},
}

func makeRecordData(listData []string) []interface{} {
	recordData := make([]interface{}, 0)
	for _, data := range listData {
		recordData = append(recordData, data)
	}
	return recordData
}

func NewMockBizflyCloudClient() *mockBizflyCloudClient {
	return &mockBizflyCloudClient{
		Zones: map[string]string{
			"Z001": "bar.com",
			"Z002": "foo.com",
		},
		Records: map[string]gobizfly.Record{
			"R001": {},
			"R002": {},
		},
	}
}

func NewMockBizflyCloudClientWithRecords(records []gobizfly.Record) *mockBizflyCloudClient {
	m := NewMockBizflyCloudClient()

	for _, record := range records {
		m.Records[record.ID] = record
	}

	return m
}

func getDNSRecordFromRecordParams(crpl interface{}, zoneID string, recordID string) gobizfly.Record {
	switch params := crpl.(type) {
	case gobizfly.CreateNormalRecordPayload:
		return gobizfly.Record{
			Name:   params.Name,
			TTL:    params.TTL,
			Type:   params.Type,
			ZoneID: zoneID,
			Data:   makeRecordData(params.Data),
		}
	case gobizfly.UpdateNormalRecordPayload:
		return gobizfly.Record{
			ID:     recordID,
			Name:   params.Name,
			TTL:    params.TTL,
			Type:   params.Type,
			ZoneID: zoneID,
			Data:   makeRecordData(params.Data),
		}
	default:
		return gobizfly.Record{}
	}
}

func (m *mockBizflyCloudClient) CreateRecord(ctx context.Context, zoneID string, crpl interface{}) (*gobizfly.Record, error) {
	recordData := getDNSRecordFromRecordParams(crpl, zoneID, "")
	m.Actions = append(m.Actions, MockAction{
		Name:       "Create",
		ZoneId:     zoneID,
		RecordData: recordData,
	})
	m.Records["R003"] = recordData
	return &gobizfly.Record{}, nil
}

func (m *mockBizflyCloudClient) UpdateRecord(ctx context.Context, recordID string, urpl interface{}) (*gobizfly.Record, error) {
	if record, ok := m.Records[recordID]; ok {
		zoneID := record.ZoneID
		recordData := getDNSRecordFromRecordParams(urpl, zoneID, recordID)
		m.Actions = append(m.Actions, MockAction{
			Name:       "Update",
			ZoneId:     zoneID,
			RecordData: recordData,
		})
		return &gobizfly.Record{}, nil
	}
	return nil, errors.New("Unknown zoneID: " + recordID)
}

func (m *mockBizflyCloudClient) DeleteRecord(ctx context.Context, recordID string) error {
	if record, ok := m.Records[recordID]; ok {
		zoneID := record.ZoneID
		m.Actions = append(m.Actions, MockAction{
			Name:   "Delete",
			ZoneId: zoneID,
			RecordData: gobizfly.Record{
				ID: record.ID,
			},
		})
		delete(m.Records, recordID)
	}
	return nil
}

func (m *mockBizflyCloudClient) ListZones(ctx context.Context, opts *gobizfly.ListOptions) (*gobizfly.ListZoneResp, error) {
	result := gobizfly.ListZoneResp{}

	for zoneID, zoneName := range m.Zones {
		result.Zones = append(result.Zones, gobizfly.Zone{
			ID:   zoneID,
			Name: zoneName,
		})
	}

	return &result, nil
}

func (m *mockBizflyCloudClient) GetZone(ctx context.Context, zoneID string) (*gobizfly.ExtendedZone, error) {
	recordSet := []gobizfly.Record{}
	for _, record := range m.Records {
		if record.ZoneID == zoneID {
			recordSet = append(recordSet, record)
		}
	}
	for id, zoneName := range m.Zones {
		if zoneID == id {
			return &gobizfly.ExtendedZone{
				Zone: gobizfly.Zone{
					ID:   zoneID,
					Name: zoneName,
				},
				RecordsSet: recordSet,
			}, nil
		}
	}

	return &gobizfly.ExtendedZone{}, errors.New("Unknown zoneID: " + zoneID)
}

func TestBizflycloudZones(t *testing.T) {
	provider := &BizflyCloudProvider{
		Client:       NewMockBizflyCloudClient(),
		domainFilter: endpoint.NewDomainFilter([]string{"bar.com"}),
	}

	zones, err := provider.listDNSZonesWithAutoPagination(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(zones))
	assert.Equal(t, "bar.com", zones[0].Name)
}

func TestBizflyCloudZonesWithIDFilter(t *testing.T) {
	client := NewMockBizflyCloudClient()
	provider := &BizflyCloudProvider{
		Client:       client,
		domainFilter: endpoint.NewDomainFilter([]string{"bar.com"}),
	}

	zones, err := provider.listDNSZonesWithAutoPagination(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// foo.com should *not* be returned as it doesn't match ZoneID filter
	assert.Equal(t, 1, len(zones))
	assert.Equal(t, "bar.com", zones[0].Name)
}

func TestBizflycloudRecords(t *testing.T) {
	client := NewMockBizflyCloudClientWithRecords(ExampleRecrods)

	// Set DNSRecordsPerPage to 1 test the pagination behaviour
	provider := &BizflyCloudProvider{
		Client:       client,
		domainFilter: endpoint.NewDomainFilter([]string{"bar.com"}),
	}
	ctx := context.Background()

	records, err := provider.Records(ctx)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	assert.Equal(t, 2, len(records))
}

func TestBizflycloudProvider(t *testing.T) {
	config := Configuration{
		APICredentialId:     "e5d084f79fd5407da705f8df97332090",
		APICredentialSecret: "Fhkp66ClGHPFTnjHQId1RWrUoG14qMIK8IWT4GxURTNLfY2cAkVmpTwyJ-xYsDlEqrS0lqBSGG8DEGZma6OQgQ",
		Debug:               false,
		DryRun:              false,
		Region:              "HN",
		APIPageSize:         100,
	}
	_, err := NewBizflyCloudProvider(
		endpoint.NewDomainFilter([]string{"bar.com"}),
		&config)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	emptyConfig := Configuration{}
	_, err = NewBizflyCloudProvider(
		endpoint.NewDomainFilter([]string{"bar.com"}),
		&emptyConfig)
	if err == nil {
		t.Errorf("expected to fail")
	}

}

func TestBizflycloudApplyChanges(t *testing.T) {
	changes := &plan.Changes{}
	client := NewMockBizflyCloudClientWithRecords(ExampleRecrods)

	provider := &BizflyCloudProvider{
		Client: client,
	}

	changes.Create = []*endpoint.Endpoint{{
		DNSName:    "new.bar.com",
		RecordTTL:  60,
		RecordType: "A",
		Targets:    endpoint.Targets{"target1", "target2"},
	}, {
		DNSName:    "new.ext-dns-test.unrelated.to",
		RecordTTL:  60,
		RecordType: "A",
		Targets:    endpoint.Targets{"target"},
	}}
	changes.Delete = []*endpoint.Endpoint{{
		DNSName:    "foobar.bar.com",
		RecordTTL:  60,
		RecordType: "A",
		Targets:    endpoint.Targets{"target"},
	}}
	changes.UpdateOld = []*endpoint.Endpoint{{
		DNSName:    "foobar.bar.com",
		RecordTTL:  60,
		RecordType: "A",
		Targets:    endpoint.Targets{"target-old"},
	}}
	changes.UpdateNew = []*endpoint.Endpoint{{
		DNSName:    "foobar.bar.com",
		RecordTTL:  60,
		RecordType: "A",
		Targets:    endpoint.Targets{"target-new"},
	}}
	err := provider.ApplyChanges(context.Background(), changes)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}

	td.Cmp(t, client.Actions, []MockAction{
		{
			Name:   "Create",
			ZoneId: "Z001",
			RecordData: gobizfly.Record{
				Name:   "new.bar.com",
				ZoneID: "Z001",
				Type:   "A",
				TTL:    60,
				Data:   makeRecordData(endpoint.Targets{"target1", "target2"}),
			},
		},
		{
			Name:   "Update",
			ZoneId: "Z001",
			RecordData: gobizfly.Record{
				Name:   "foobar.bar.com",
				ZoneID: "Z001",
				Type:   "A",
				TTL:    60,
				ID:     "R001",
				Data:   makeRecordData([]string{"target-new"}),
			},
		},
		{
			Name:   "Delete",
			ZoneId: "Z001",
			RecordData: gobizfly.Record{
				ID: "R001",
			},
		},
	})

	// empty changes
	changes.Create = []*endpoint.Endpoint{}
	changes.Delete = []*endpoint.Endpoint{}
	changes.UpdateOld = []*endpoint.Endpoint{}
	changes.UpdateNew = []*endpoint.Endpoint{}

	err = provider.ApplyChanges(context.Background(), changes)
	if err != nil {
		t.Errorf("should not fail, %s", err)
	}
}

func TestBizflycloudGetRecordID(t *testing.T) {
	p := &BizflyCloudProvider{}
	records := []gobizfly.Record{
		{
			ID:     "1",
			Name:   "cname",
			Type:   endpoint.RecordTypeCNAME,
			ZoneID: "Z001",
			Data:   makeRecordData([]string{"foo.bar.com"}),
		},
		{
			ID:     "2",
			Name:   "@",
			Type:   endpoint.RecordTypeA,
			ZoneID: "Z001",
			Data:   makeRecordData([]string{"1.2.3.4"}),
		},
		{
			ID:     "3",
			Name:   "foo",
			Type:   endpoint.RecordTypeA,
			ZoneID: "Z001",
			Data:   makeRecordData([]string{"1.2.3.4"}),
		},
	}
	zone := gobizfly.ExtendedZone{
		Zone: gobizfly.Zone{
			Name: "bar.com",
		},
		RecordsSet: records,
	}

	assert.Equal(t, "", p.getRecordID(&zone, NormalRecord{
		Name: "bar.com",
		Type: endpoint.RecordTypeCNAME,
	}))

	assert.Equal(t, "", p.getRecordID(&zone, NormalRecord{
		Name: "cname",
		Type: endpoint.RecordTypeA,
	}))

	assert.Equal(t, "1", p.getRecordID(&zone, NormalRecord{
		Name: "cname.bar.com",
		Type: endpoint.RecordTypeCNAME,
	}))
	assert.Equal(t, "2", p.getRecordID(&zone, NormalRecord{
		Name: "bar.com",
		Type: endpoint.RecordTypeA,
	}))
	assert.Equal(t, "3", p.getRecordID(&zone, NormalRecord{
		Name: "foo.bar.com",
		Type: endpoint.RecordTypeA,
	}))
}
