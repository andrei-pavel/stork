package restservice

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	keaconfig "isc.org/stork/appcfg/kea"
	keactrl "isc.org/stork/appctrl/kea"
	"isc.org/stork/pki"
	"isc.org/stork/server/agentcomm"
	agentcommtest "isc.org/stork/server/agentcomm/test"
	"isc.org/stork/server/apps/kea"
	"isc.org/stork/server/certs"
	dbops "isc.org/stork/server/database"
	dbmodel "isc.org/stork/server/database/model"
	dbtest "isc.org/stork/server/database/test"
	"isc.org/stork/server/gen/models"
	dhcp "isc.org/stork/server/gen/restapi/operations/d_h_c_p"
	"isc.org/stork/server/gen/restapi/operations/general"
	"isc.org/stork/server/gen/restapi/operations/services"
	storktest "isc.org/stork/server/test/dbmodel"
	"isc.org/stork/testutil"
	storkutil "isc.org/stork/util"
)

func TestGetVersion(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	params := general.GetVersionParams{}

	rsp := rapi.GetVersion(ctx, params)
	p := rsp.(*general.GetVersionOK).Payload
	require.Regexp(t, `^\d+.\d+.\d+$`, *p.Version)
	require.Equal(t, "unset", *p.Date)
}

func TestGetMachineStateOnly(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// get state of non-existing machine
	params := services.GetMachineStateParams{
		ID: 123,
	}
	rsp := rapi.GetMachineState(ctx, params)
	require.IsType(t, &services.GetMachineStateDefault{}, rsp)
	defaultRsp := rsp.(*services.GetMachineStateDefault)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
	require.Equal(t, "Cannot find machine with ID 123", *defaultRsp.Payload.Message)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// get added machine
	params = services.GetMachineStateParams{
		ID: m.ID,
	}
	rsp = rapi.GetMachineState(ctx, params)
	require.IsType(t, &services.GetMachineStateOK{}, rsp)
	okRsp := rsp.(*services.GetMachineStateOK)
	require.Equal(t, "localhost", *okRsp.Payload.Address)
	require.EqualValues(t, 8080, okRsp.Payload.AgentPort)
	require.Less(t, int64(0), okRsp.Payload.Memory)
	require.Less(t, int64(0), okRsp.Payload.Cpus)
	require.LessOrEqual(t, int64(0), okRsp.Payload.Uptime)
	require.NotNil(t, okRsp.Payload.Apps)
	require.Len(t, okRsp.Payload.Apps, 0)
}

func mockGetAppsState(callNo int, cmdResponses []interface{}) {
	switch callNo {
	case 0:
		list1 := cmdResponses[0].(*[]kea.VersionGetResponse)
		*list1 = []kea.VersionGetResponse{
			{
				ResponseHeader: keactrl.ResponseHeader{
					Result: 0,
					Daemon: "ca",
				},
				Arguments: &kea.VersionGetRespArgs{
					Extended: "Extended version",
				},
			},
		}
		list2 := cmdResponses[1].(*[]keactrl.HashedResponse)
		*list2 = []keactrl.HashedResponse{
			{
				ResponseHeader: keactrl.ResponseHeader{
					Result: 0,
					Daemon: "ca",
				},
				Arguments: &map[string]interface{}{
					"Control-agent": map[string]interface{}{
						"control-sockets": map[string]interface{}{
							"dhcp4": map[string]interface{}{
								"socket-name": "aaa",
								"socket-type": "unix",
							},
						},
					},
				},
			},
		}
	case 1:
		// version-get response
		list1 := cmdResponses[0].(*[]kea.VersionGetResponse)
		*list1 = []kea.VersionGetResponse{
			{
				ResponseHeader: keactrl.ResponseHeader{
					Result: 0,
					Daemon: "dhcp4",
				},
				Arguments: &kea.VersionGetRespArgs{
					Extended: "Extended version",
				},
			},
		}
		// status-get response
		list2 := cmdResponses[1].(*[]kea.StatusGetResponse)
		*list2 = []kea.StatusGetResponse{
			{
				ResponseHeader: keactrl.ResponseHeader{
					Result: 0,
					Daemon: "dhcp4",
				},
				Arguments: &kea.StatusGetRespArgs{
					Pid: 123,
				},
			},
		}
		// config-get response
		list3 := cmdResponses[2].(*[]keactrl.HashedResponse)
		*list3 = []keactrl.HashedResponse{
			{
				ResponseHeader: keactrl.ResponseHeader{
					Result: 0,
					Daemon: "dhcp4",
				},
				Arguments: &map[string]interface{}{
					"Dhcp4": map[string]interface{}{
						"hooks-libraries": []interface{}{
							map[string]interface{}{
								"library": "hook_abc.so",
							},
							map[string]interface{}{
								"library": "hook_def.so",
							},
						},
					},
				},
			},
		}
	}
}

func TestGetMachineAndAppsState(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(mockGetAppsState, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// add kea app
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "1.2.3.4", "", 123, false)
	keaApp := &dbmodel.App{
		MachineID:    m.ID,
		Machine:      m,
		Type:         dbmodel.AppTypeKea,
		AccessPoints: keaPoints,
	}
	_, err = dbmodel.AddApp(db, keaApp)
	require.NoError(t, err)
	m.Apps = append(m.Apps, keaApp)

	// add BIND 9 app
	var bind9Points []*dbmodel.AccessPoint
	bind9Points = dbmodel.AppendAccessPoint(bind9Points, dbmodel.AccessPointControl, "1.2.3.4", "abcd", 124, true)
	bind9App := &dbmodel.App{
		MachineID:    m.ID,
		Machine:      m,
		Type:         dbmodel.AppTypeBind9,
		AccessPoints: bind9Points,
	}
	_, err = dbmodel.AddApp(db, bind9App)
	require.NoError(t, err)
	m.Apps = append(m.Apps, bind9App)

	fa.MachineState = &agentcomm.State{
		Apps: []*agentcomm.App{
			{
				Type:         dbmodel.AppTypeKea.String(),
				AccessPoints: agentcomm.MakeAccessPoint(dbmodel.AccessPointControl, "1.2.3.4", "", 123),
			},
			{
				Type:         dbmodel.AppTypeBind9.String(),
				AccessPoints: agentcomm.MakeAccessPoint(dbmodel.AccessPointControl, "1.2.3.4", "abcd", 124),
			},
		},
	}

	// get added machine
	params := services.GetMachineStateParams{
		ID: m.ID,
	}
	rsp := rapi.GetMachineState(ctx, params)
	require.IsType(t, &services.GetMachineStateOK{}, rsp)
	okRsp := rsp.(*services.GetMachineStateOK)
	require.Len(t, okRsp.Payload.Apps, 2)
	require.Equal(t, dbmodel.AppTypeKea.String(), okRsp.Payload.Apps[0].Type)
	require.Equal(t, dbmodel.AppTypeBind9.String(), okRsp.Payload.Apps[1].Type)
	require.Nil(t, okRsp.Payload.LastVisitedAt)
}

func TestCreateMachine(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// empty request, variant 1 - should raise an error
	params := services.CreateMachineParams{}
	rsp := rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp := rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Missing parameters", *defaultRsp.Payload.Message)

	// empty request, variant 2 - should raise an error
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Missing parameters", *defaultRsp.Payload.Message)

	// prepare request arguments
	addr := "1.2.3.4"
	port := int64(8080)
	serverToken := "serverToken" // it will be corrected later when server cert is generated
	agentToken := "agentToken"
	privKeyPEM, err := pki.GenKey()
	require.NoError(t, err)
	csrPEM, _, err := pki.GenCSRUsingKey("agent", []string{"name"}, []net.IP{net.ParseIP("192.0.2.1")}, privKeyPEM)
	require.NoError(t, err)
	agentCSR := string(csrPEM)

	// bad host
	badAddr := "//a/"
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &badAddr,
			AgentPort:   port,
			AgentCSR:    &agentCSR,
			ServerToken: serverToken,
			AgentToken:  &agentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Cannot parse address", *defaultRsp.Payload.Message)

	// bad port
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &addr,
			AgentPort:   0,
			AgentCSR:    &agentCSR,
			ServerToken: serverToken,
			AgentToken:  &agentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Bad port", *defaultRsp.Payload.Message)

	// missing server cert in db so there will be server internal error
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &addr,
			AgentPort:   port,
			AgentCSR:    &agentCSR,
			ServerToken: serverToken,
			AgentToken:  &agentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusInternalServerError, getStatusCode(*defaultRsp))
	require.Equal(t, "Server internal problem - server token is empty", *defaultRsp.Payload.Message)

	// add server cert to db
	caCertPEM1, serverCertPEM1, serverKeyPEM1, err := certs.SetupServerCerts(db)
	require.NoError(t, err)

	// check if rerun of setup server certs works as well
	caCertPEM2, serverCertPEM2, serverKeyPEM2, err := certs.SetupServerCerts(db)
	require.NoError(t, err)
	require.Equal(t, caCertPEM1, caCertPEM2)
	require.Equal(t, serverCertPEM1, serverCertPEM2)
	require.Equal(t, serverKeyPEM1, serverKeyPEM2)

	// get server token to use it in requests below
	dbServerToken, err := dbmodel.GetSecret(db, dbmodel.SecretServerToken)
	require.NoError(t, err)
	serverToken = string(dbServerToken) // set correct value for server token

	// bad server token
	badServerToken := "abc"
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &addr,
			AgentPort:   port,
			AgentCSR:    &agentCSR,
			ServerToken: badServerToken,
			AgentToken:  &agentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Provided server token is wrong", *defaultRsp.Payload.Message)

	// bad agent CSR
	badAgentCSR := "abc"
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &addr,
			AgentPort:   port,
			AgentCSR:    &badAgentCSR,
			ServerToken: serverToken,
			AgentToken:  &agentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Problem with agent CSR", *defaultRsp.Payload.Message)

	// bad (empty) agent token
	emptyAgentToken := ""
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &addr,
			AgentPort:   8080,
			AgentCSR:    &agentCSR,
			ServerToken: serverToken,
			AgentToken:  &emptyAgentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.CreateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Agent token cannot be empty", *defaultRsp.Payload.Message)

	// all ok
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &addr,
			AgentPort:   8080,
			AgentCSR:    &agentCSR,
			ServerToken: serverToken,
			AgentToken:  &agentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineOK{}, rsp)
	okRsp := rsp.(*services.CreateMachineOK)
	require.NotEmpty(t, okRsp.Payload.ID)
	require.NotEmpty(t, okRsp.Payload.ServerCACert)
	require.NotEmpty(t, okRsp.Payload.AgentCert)
	machines, err := dbmodel.GetAllMachines(db, nil)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	m1 := machines[0]
	require.True(t, m1.Authorized)
	certFingerprint1 := m1.CertFingerprint

	serverCACertFingerprint, err := pki.CalculateFingerprintFromPEM([]byte(okRsp.Payload.ServerCACert))
	require.NoError(t, err)

	// ok, now lets ping the machine if it is alive
	pingParams := services.PingMachineParams{
		ID: m1.ID,
		Ping: services.PingMachineBody{
			ServerToken: serverToken,
			AgentToken:  agentToken,
		},
	}
	require.False(t, fa.GetStateCalled)
	pingRsp := rapi.PingMachine(ctx, pingParams)
	require.IsType(t, &services.PingMachineOK{}, pingRsp)
	_, ok := pingRsp.(*services.PingMachineOK)
	require.True(t, ok)
	// check if GetMachineAndAppsState was called
	require.True(t, fa.GetStateCalled)

	// re-register (the same) machine (it should be break)
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:           &addr,
			AgentPort:         8080,
			AgentCSR:          &agentCSR,
			ServerToken:       serverToken,
			AgentToken:        &agentToken,
			CaCertFingerprint: storkutil.BytesToHex(serverCACertFingerprint[:]),
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineConflict{}, rsp)
	conflictRsp := rsp.(*services.CreateMachineConflict)
	require.NotEmpty(t, conflictRsp.Location)
	expectedLocation := fmt.Sprintf("/machines/%d", okRsp.Payload.ID)
	require.Equal(t, expectedLocation, conflictRsp.Location)

	machines, err = dbmodel.GetAllMachines(db, nil)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	m1 = machines[0]
	require.True(t, m1.Authorized)
	// agent cert isn't re-signed so fingerprint should be the same
	require.Equal(t, certFingerprint1, m1.CertFingerprint)
	require.True(t, m1.LastVisitedAt.IsZero())

	// add another machine but with no server token (agent token is used for authorization)
	addr = "5.6.7.8"
	serverToken = ""
	params = services.CreateMachineParams{
		Machine: &models.NewMachineReq{
			Address:     &addr,
			AgentPort:   8080,
			AgentCSR:    &agentCSR,
			ServerToken: serverToken,
			AgentToken:  &agentToken,
		},
	}
	rsp = rapi.CreateMachine(ctx, params)
	require.IsType(t, &services.CreateMachineOK{}, rsp)
	okRsp = rsp.(*services.CreateMachineOK)
	require.NotEmpty(t, okRsp.Payload.ID)
	require.NotEmpty(t, okRsp.Payload.ServerCACert)
	require.NotEmpty(t, okRsp.Payload.AgentCert)
	machines, err = dbmodel.GetAllMachines(db, nil)
	require.NoError(t, err)
	require.Len(t, machines, 2)
	m2 := machines[0]
	if m2.ID == m1.ID {
		m2 = machines[1]
	}
	require.False(t, m2.Authorized)
}

func TestGetMachines(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	machine1 := &dbmodel.Machine{
		Address:   "machine1.example.org",
		AgentPort: 8080,
	}
	err := dbmodel.AddMachine(db, machine1)
	require.NoError(t, err)

	machine2 := &dbmodel.Machine{
		Address:   "machine2.example.org",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, machine2)
	require.NoError(t, err)

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	var start, limit int64 = 0, 10
	params := services.GetMachinesParams{
		Start: &start,
		Limit: &limit,
	}

	rsp := rapi.GetMachines(ctx, params)
	ms := rsp.(*services.GetMachinesOK).Payload
	require.EqualValues(t, ms.Total, 2)
}

// Test that the empty list is returned when the database contains no machines.
func TestGetMachinesEmptyList(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	var start, limit int64 = 0, 10
	params := services.GetMachinesParams{
		Start: &start,
		Limit: &limit,
	}

	rsp := rapi.GetMachines(ctx, params)
	ms := rsp.(*services.GetMachinesOK).Payload
	require.EqualValues(t, ms.Total, 0)
	require.NotNil(t, ms.Items)
	require.Len(t, ms.Items, 0)
}

// Test that a list of machines' ids and addresses/names is returned
// via the API.
func TestGetMachinesDirectory(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	// Add machines.
	machine1 := &dbmodel.Machine{
		Address:    "machine1.example.org",
		AgentPort:  8080,
		Authorized: true,
	}
	err := dbmodel.AddMachine(db, machine1)
	require.NoError(t, err)

	machine2 := &dbmodel.Machine{
		Address:    "machine2.example.org",
		AgentPort:  8080,
		Authorized: true,
	}
	err = dbmodel.AddMachine(db, machine2)
	require.NoError(t, err)

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	params := services.GetMachinesDirectoryParams{}

	rsp := rapi.GetMachinesDirectory(ctx, params)
	machines := rsp.(*services.GetMachinesDirectoryOK).Payload
	require.EqualValues(t, machines.Total, 2)

	// Ensure that the returned machines are in a coherent order.
	sort.Slice(machines.Items, func(i, j int) bool {
		return machines.Items[i].ID < machines.Items[j].ID
	})

	// Validate the returned data.
	require.Equal(t, machine1.ID, machines.Items[0].ID)
	require.NotNil(t, machines.Items[0].Address)
	require.Equal(t, machine1.Address, *machines.Items[0].Address)
	require.Equal(t, machine2.ID, machines.Items[1].ID)
	require.NotNil(t, machines.Items[1].Address)
	require.Equal(t, machine2.Address, *machines.Items[1].Address)
}

func TestGetMachine(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// get non-existing machine
	params := services.GetMachineParams{
		ID: 123,
	}
	rsp := rapi.GetMachine(ctx, params)
	require.IsType(t, &services.GetMachineDefault{}, rsp)
	defaultRsp := rsp.(*services.GetMachineDefault)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
	require.Equal(t, "Cannot find machine with ID 123", *defaultRsp.Payload.Message)

	// add machine
	m := &dbmodel.Machine{
		Address:       "localhost",
		AgentPort:     8080,
		LastVisitedAt: time.Now(),
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// get added machine
	params = services.GetMachineParams{
		ID: m.ID,
	}
	rsp = rapi.GetMachine(ctx, params)
	require.IsType(t, &services.GetMachineOK{}, rsp)
	okRsp := rsp.(*services.GetMachineOK)
	require.Equal(t, m.ID, okRsp.Payload.ID)
	require.NotNil(t, okRsp.Payload.LastVisitedAt)

	// add machine 2
	m2 := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8082,
	}
	err = dbmodel.AddMachine(db, m2)
	require.NoError(t, err)

	// add app to machine 2
	var accessPoints []*dbmodel.AccessPoint
	accessPoints = dbmodel.AppendAccessPoint(accessPoints, dbmodel.AccessPointControl, "", "", 1234, false)
	s := &dbmodel.App{
		ID:           0,
		MachineID:    m2.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: accessPoints,
		Daemons:      []*dbmodel.Daemon{},
	}
	_, err = dbmodel.AddApp(db, s)
	require.NoError(t, err)
	require.NotEqual(t, 0, s.ID)

	// get added machine 2 with kea app
	params = services.GetMachineParams{
		ID: m2.ID,
	}
	rsp = rapi.GetMachine(ctx, params)
	require.IsType(t, &services.GetMachineOK{}, rsp)
	okRsp = rsp.(*services.GetMachineOK)
	require.Equal(t, m2.ID, okRsp.Payload.ID)
	require.Len(t, okRsp.Payload.Apps, 1)
	require.Equal(t, s.ID, okRsp.Payload.Apps[0].ID)
	require.Len(t, okRsp.Payload.Apps[0].AccessPoints, 1)
	require.Nil(t, okRsp.Payload.LastVisitedAt)
}

func TestUpdateMachine(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// empty request, variant 1 - should raise an error
	params := services.UpdateMachineParams{}
	rsp := rapi.UpdateMachine(ctx, params)
	defaultRsp := rsp.(*services.UpdateMachineDefault)
	require.IsType(t, &services.UpdateMachineDefault{}, rsp)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Missing parameters", *defaultRsp.Payload.Message)

	// empty request, variant 2 - should raise an error
	params = services.UpdateMachineParams{
		Machine: &models.Machine{},
	}
	rsp = rapi.UpdateMachine(ctx, params)
	require.IsType(t, &services.UpdateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.UpdateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Missing parameters", *defaultRsp.Payload.Message)

	// update non-existing machine
	addr := "localhost"
	params = services.UpdateMachineParams{
		ID: 123,
		Machine: &models.Machine{
			Address:   &addr,
			AgentPort: 8080,
		},
	}
	rsp = rapi.UpdateMachine(ctx, params)
	require.IsType(t, &services.UpdateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.UpdateMachineDefault)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
	require.Equal(t, "Cannot find machine with ID 123", *defaultRsp.Payload.Message)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 1010,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// update added machine - all ok
	params = services.UpdateMachineParams{
		ID: m.ID,
		Machine: &models.Machine{
			Address:   &addr,
			AgentPort: 8080,
		},
	}
	rsp = rapi.UpdateMachine(ctx, params)
	okRsp := rsp.(*services.UpdateMachineOK)
	require.Equal(t, m.ID, okRsp.Payload.ID)
	require.Equal(t, addr, *okRsp.Payload.Address)
	require.False(t, okRsp.Payload.Authorized) // machine is not yet authorized
	require.Nil(t, okRsp.Payload.LastVisitedAt)

	// setup a user session, it is required to check user role in UpdateMachine
	// in case of authorization change
	user, err := dbmodel.GetUserByID(rapi.DB, 1)
	require.NoError(t, err)
	ctx2, err := rapi.SessionManager.Load(ctx, "")
	require.NoError(t, err)
	err = rapi.SessionManager.LoginHandler(ctx2, user)
	require.NoError(t, err)

	// authorize the machine
	require.False(t, fa.GetStateCalled)
	params = services.UpdateMachineParams{
		ID: m.ID,
		Machine: &models.Machine{
			Address:    &addr,
			AgentPort:  8080,
			Authorized: true,
		},
	}
	rsp = rapi.UpdateMachine(ctx2, params)
	okRsp = rsp.(*services.UpdateMachineOK)
	require.Equal(t, m.ID, okRsp.Payload.ID)
	require.Equal(t, addr, *okRsp.Payload.Address)
	require.True(t, okRsp.Payload.Authorized) // machine is authorized now
	// check if GetMachineAndAppsState was called because it was just authorized
	require.True(t, fa.GetStateCalled)

	// add another machine
	m2 := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 2020,
	}
	err = dbmodel.AddMachine(db, m2)
	require.NoError(t, err)

	// update second machine to have the same address - should raise error due to duplication
	params = services.UpdateMachineParams{
		ID: m2.ID,
		Machine: &models.Machine{
			Address:   &addr,
			AgentPort: 8080,
		},
	}
	rsp = rapi.UpdateMachine(ctx, params)
	require.IsType(t, &services.UpdateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.UpdateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Machine with address localhost:8080 already exists", *defaultRsp.Payload.Message)

	// update second machine with bad address
	addr = "aaa:"
	params = services.UpdateMachineParams{
		ID: m2.ID,
		Machine: &models.Machine{
			Address:   &addr,
			AgentPort: 8080,
		},
	}
	rsp = rapi.UpdateMachine(ctx, params)
	require.IsType(t, &services.UpdateMachineDefault{}, rsp)
	defaultRsp = rsp.(*services.UpdateMachineDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
	require.Equal(t, "Cannot parse address", *defaultRsp.Payload.Message)
}

func TestDeleteMachine(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// delete non-existing machine
	params := services.DeleteMachineParams{
		ID: 123,
	}
	rsp := rapi.DeleteMachine(ctx, params)
	require.IsType(t, &services.DeleteMachineOK{}, rsp)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost:1010",
		AgentPort: 1010,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// get added machine
	params2 := services.GetMachineParams{
		ID: m.ID,
	}
	rsp = rapi.GetMachine(ctx, params2)
	require.IsType(t, &services.GetMachineOK{}, rsp)
	okRsp := rsp.(*services.GetMachineOK)
	require.Equal(t, m.ID, okRsp.Payload.ID)

	// delete added machine
	params = services.DeleteMachineParams{
		ID: m.ID,
	}
	rsp = rapi.DeleteMachine(ctx, params)
	require.IsType(t, &services.DeleteMachineOK{}, rsp)

	// get deleted machine - should return not found
	params2 = services.GetMachineParams{
		ID: m.ID,
	}
	rsp = rapi.GetMachine(ctx, params2)
	require.IsType(t, &services.GetMachineDefault{}, rsp)
	defaultRsp := rsp.(*services.GetMachineDefault)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
}

func TestGetApp(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// get non-existing app
	params := services.GetAppParams{
		ID: 123,
	}
	rsp := rapi.GetApp(ctx, params)
	require.IsType(t, &services.GetAppDefault{}, rsp)
	defaultRsp := rsp.(*services.GetAppDefault)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
	require.Equal(t, "Cannot find app with ID 123", *defaultRsp.Payload.Message)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// add app to machine
	var accessPoints []*dbmodel.AccessPoint
	accessPoints = dbmodel.AppendAccessPoint(accessPoints, dbmodel.AccessPointControl, "", "", 1234, true)
	s := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: accessPoints,
		Daemons:      []*dbmodel.Daemon{},
	}
	_, err = dbmodel.AddApp(db, s)
	require.NoError(t, err)

	// get added app
	params = services.GetAppParams{
		ID: s.ID,
	}
	rsp = rapi.GetApp(ctx, params)
	require.IsType(t, &services.GetAppOK{}, rsp)
	okRsp := rsp.(*services.GetAppOK)
	require.Equal(t, s.ID, okRsp.Payload.ID)
}

func TestRestGetApp(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// get non-existing app
	params := services.GetAppParams{
		ID: 123,
	}
	rsp := rapi.GetApp(ctx, params)
	require.IsType(t, &services.GetAppDefault{}, rsp)
	defaultRsp := rsp.(*services.GetAppDefault)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
	require.Equal(t, "Cannot find app with ID 123", *defaultRsp.Payload.Message)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// add kea app to machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "", "", 1234, false)
	keaApp := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		Name:         "fancy-app",
		AccessPoints: keaPoints,
		Daemons:      []*dbmodel.Daemon{},
	}
	_, err = dbmodel.AddApp(db, keaApp)
	require.NoError(t, err)

	// get added kea app
	params = services.GetAppParams{
		ID: keaApp.ID,
	}
	rsp = rapi.GetApp(ctx, params)
	require.IsType(t, &services.GetAppOK{}, rsp)
	okRsp := rsp.(*services.GetAppOK)
	require.Equal(t, keaApp.ID, okRsp.Payload.ID)
	require.Equal(t, keaApp.Name, okRsp.Payload.Name)

	// add BIND 9 app to machine
	var bind9Points []*dbmodel.AccessPoint
	bind9Points = dbmodel.AppendAccessPoint(bind9Points, dbmodel.AccessPointControl, "", "abcd", 953, true)
	bind9App := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeBind9,
		Active:       true,
		Name:         "another-fancy-app",
		AccessPoints: bind9Points,
		Daemons: []*dbmodel.Daemon{
			{
				Bind9Daemon: &dbmodel.Bind9Daemon{},
			},
		},
	}
	_, err = dbmodel.AddApp(db, bind9App)
	require.NoError(t, err)

	// get added BIND 9 app
	params = services.GetAppParams{
		ID: bind9App.ID,
	}
	rsp = rapi.GetApp(ctx, params)
	require.IsType(t, &services.GetAppOK{}, rsp)
	okRsp = rsp.(*services.GetAppOK)
	require.Equal(t, bind9App.ID, okRsp.Payload.ID)
	require.Equal(t, bind9App.Name, okRsp.Payload.Name)
}

func TestRestGetApps(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// get empty list of app
	params := services.GetAppsParams{}
	rsp := rapi.GetApps(ctx, params)
	require.IsType(t, &services.GetAppsOK{}, rsp)
	okRsp := rsp.(*services.GetAppsOK)
	require.Zero(t, okRsp.Payload.Total)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// add app kea to machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "", "", 1234, false)
	s1 := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		Name:         "fancy-app",
		AccessPoints: keaPoints,
		Daemons: []*dbmodel.Daemon{
			{
				Name:      "dhcp4",
				KeaDaemon: &dbmodel.KeaDaemon{},
				LogTargets: []*dbmodel.LogTarget{
					{
						Name:     "kea-dhcp4",
						Severity: "DEBUG",
						Output:   "/tmp/log",
					},
				},
			},
		},
	}
	_, err = dbmodel.AddApp(db, s1)
	require.NoError(t, err)

	// add app BIND 9 to machine
	var bind9Points []*dbmodel.AccessPoint
	bind9Points = dbmodel.AppendAccessPoint(bind9Points, dbmodel.AccessPointControl, "", "abcd", 4321, true)
	s2 := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeBind9,
		Active:       true,
		Name:         "another-fancy-app",
		AccessPoints: bind9Points,
		Daemons: []*dbmodel.Daemon{
			{
				Bind9Daemon: &dbmodel.Bind9Daemon{},
			},
		},
	}
	_, err = dbmodel.AddApp(db, s2)
	require.NoError(t, err)

	// get added apps
	params = services.GetAppsParams{}
	rsp = rapi.GetApps(ctx, params)
	require.IsType(t, &services.GetAppsOK{}, rsp)
	okRsp = rsp.(*services.GetAppsOK)
	require.EqualValues(t, 2, okRsp.Payload.Total)

	// Verify that the communication error counters are returned. See fake_agents.go
	// to see where those counters are set.
	require.Len(t, okRsp.Payload.Items, 2)
	for _, app := range okRsp.Payload.Items {
		if app.Type == dbmodel.AppTypeKea.String() {
			require.Equal(t, "fancy-app", app.Name)
			appKea := app.Details.AppKea
			require.Len(t, appKea.Daemons, 1)
			daemon := appKea.Daemons[0]
			require.EqualValues(t, 1, daemon.AgentCommErrors)
			require.EqualValues(t, 2, daemon.CaCommErrors)
			require.EqualValues(t, 5, daemon.DaemonCommErrors)
			require.Len(t, daemon.LogTargets, 1)
			require.Equal(t, "kea-dhcp4", daemon.LogTargets[0].Name)
			require.Equal(t, "debug", daemon.LogTargets[0].Severity)
			require.Equal(t, "/tmp/log", daemon.LogTargets[0].Output)
		} else if app.Type == dbmodel.AppTypeBind9.String() {
			require.Equal(t, "another-fancy-app", app.Name)
			appBind9 := app.Details.AppBind9
			daemon := appBind9.Daemon
			require.EqualValues(t, 1, daemon.AgentCommErrors)
			require.EqualValues(t, 2, daemon.RndcCommErrors)
			require.EqualValues(t, 3, daemon.StatsCommErrors)
		}
	}
}

// Test that a list of apps' ids and names is returned via the API.
func TestGetAppsDirectory(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// Add a machine,
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// Add Kea app to the machine.
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "", "", 1234, false)
	keaApp := &dbmodel.App{
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		Name:         "kea-app",
		AccessPoints: keaPoints,
	}
	_, err = dbmodel.AddApp(db, keaApp)
	require.NoError(t, err)

	// Add BIND 9 app to the machine.
	var bind9Points []*dbmodel.AccessPoint
	bind9Points = dbmodel.AppendAccessPoint(bind9Points, dbmodel.AccessPointControl, "", "abcd", 4321, true)
	bind9App := &dbmodel.App{
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeBind9,
		Active:       true,
		Name:         "bind9-app",
		AccessPoints: bind9Points,
	}
	_, err = dbmodel.AddApp(db, bind9App)
	require.NoError(t, err)

	params := services.GetAppsDirectoryParams{}
	rsp := rapi.GetAppsDirectory(ctx, params)
	require.IsType(t, &services.GetAppsDirectoryOK{}, rsp)
	apps := rsp.(*services.GetAppsDirectoryOK).Payload
	require.EqualValues(t, 2, apps.Total)

	// Ensure that the returned apps are in a coherent order.
	sort.Slice(apps.Items, func(i, j int) bool {
		return apps.Items[i].ID < apps.Items[j].ID
	})

	// Validate the returned data.
	require.Equal(t, keaApp.ID, apps.Items[0].ID)
	require.NotNil(t, apps.Items[0].Name)
	require.Equal(t, keaApp.Name, apps.Items[0].Name)
	require.Equal(t, bind9App.ID, apps.Items[1].ID)
	require.NotNil(t, apps.Items[1].Name)
	require.Equal(t, bind9App.Name, apps.Items[1].Name)
}

// Test that status of two HA services for a Kea application is parsed
// correctly.
func TestRestGetAppServicesStatus(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	// Configure the fake control agents to mimic returning a status of
	// two HA services for Kea.
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// Add a machine.
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// Add Kea application to the machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "127.0.0.1", "", 1234, false)
	keaApp := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: keaPoints,
		Daemons: []*dbmodel.Daemon{
			dbmodel.NewKeaDaemon("dhcp4", true),
		},
	}
	_, err = dbmodel.AddApp(db, keaApp)
	require.NoError(t, err)
	require.NotZero(t, keaApp.ID)

	exampleTime := storkutil.UTCNow().Add(-5 * time.Second)
	commInterrupted := []bool{true, true}
	keaServices := []dbmodel.Service{
		{
			BaseService: dbmodel.BaseService{
				ServiceType: "ha_dhcp",
			},
			HAService: &dbmodel.BaseHAService{
				HAType:                      "dhcp4",
				HAMode:                      "load-balancing",
				PrimaryID:                   keaApp.ID,
				PrimaryStatusCollectedAt:    exampleTime,
				SecondaryStatusCollectedAt:  exampleTime,
				PrimaryLastState:            "load-balancing",
				SecondaryLastState:          "load-balancing",
				PrimaryLastScopes:           []string{"server1"},
				SecondaryLastScopes:         []string{"server2"},
				PrimaryLastFailoverAt:       exampleTime,
				SecondaryLastFailoverAt:     exampleTime,
				PrimaryCommInterrupted:      &commInterrupted[0],
				SecondaryCommInterrupted:    &commInterrupted[1],
				PrimaryConnectingClients:    7,
				SecondaryConnectingClients:  9,
				PrimaryUnackedClients:       3,
				SecondaryUnackedClients:     4,
				PrimaryUnackedClientsLeft:   8,
				SecondaryUnackedClientsLeft: 9,
				PrimaryAnalyzedPackets:      10,
				SecondaryAnalyzedPackets:    11,
			},
		},
		{
			BaseService: dbmodel.BaseService{
				ServiceType: "ha_dhcp",
			},
			HAService: &dbmodel.BaseHAService{
				HAType:                      "dhcp6",
				HAMode:                      "hot-standby",
				PrimaryID:                   keaApp.ID,
				PrimaryStatusCollectedAt:    exampleTime,
				SecondaryStatusCollectedAt:  exampleTime,
				PrimaryLastState:            "hot-standby",
				SecondaryLastState:          "waiting",
				PrimaryLastScopes:           []string{"server1"},
				SecondaryLastScopes:         []string{},
				PrimaryLastFailoverAt:       exampleTime,
				SecondaryLastFailoverAt:     exampleTime,
				PrimaryCommInterrupted:      &commInterrupted[0],
				SecondaryCommInterrupted:    nil,
				PrimaryConnectingClients:    5,
				SecondaryConnectingClients:  0,
				PrimaryUnackedClients:       2,
				SecondaryUnackedClients:     0,
				PrimaryUnackedClientsLeft:   9,
				SecondaryUnackedClientsLeft: 0,
				PrimaryAnalyzedPackets:      123,
				SecondaryAnalyzedPackets:    0,
			},
		},
	}

	// Add the services and associate them with the app.
	for i := range keaServices {
		err = dbmodel.AddService(db, &keaServices[i])
		require.NoError(t, err)
		err = dbmodel.AddDaemonToService(db, keaServices[i].ID, keaApp.Daemons[0])
		require.NoError(t, err)
	}

	params := services.GetAppServicesStatusParams{
		ID: keaApp.ID,
	}
	rsp := rapi.GetAppServicesStatus(ctx, params)

	// Make sure that the response is ok.
	require.IsType(t, &services.GetAppServicesStatusOK{}, rsp)
	okRsp := rsp.(*services.GetAppServicesStatusOK)
	require.NotNil(t, okRsp.Payload.Items)

	// There should be two structures returned, one with a status of
	// the DHCPv4 server and one with the status of the DHCPv6 server.
	require.Len(t, okRsp.Payload.Items, 2)

	statusList := okRsp.Payload.Items

	// Validate the status of the DHCPv4 pair.
	status := statusList[0].Status.KeaStatus
	require.NotNil(t, status.HaServers)

	haStatus := status.HaServers
	require.NotNil(t, haStatus.PrimaryServer)
	require.NotNil(t, haStatus.SecondaryServer)

	require.EqualValues(t, keaApp.Daemons[0].ID, haStatus.PrimaryServer.ID)
	require.Equal(t, "primary", haStatus.PrimaryServer.Role)
	require.Len(t, haStatus.PrimaryServer.Scopes, 1)
	require.Contains(t, haStatus.PrimaryServer.Scopes, "server1")
	require.Equal(t, "load-balancing", haStatus.PrimaryServer.State)
	require.GreaterOrEqual(t, haStatus.PrimaryServer.Age, int64(5))
	require.Equal(t, "127.0.0.1", haStatus.PrimaryServer.ControlAddress)
	require.EqualValues(t, keaApp.ID, haStatus.PrimaryServer.AppID)
	require.NotEmpty(t, haStatus.PrimaryServer.StatusTime.String())
	require.EqualValues(t, 1, haStatus.PrimaryServer.CommInterrupted)
	require.EqualValues(t, 7, haStatus.PrimaryServer.ConnectingClients)
	require.EqualValues(t, 3, haStatus.PrimaryServer.UnackedClients)
	require.EqualValues(t, 8, haStatus.PrimaryServer.UnackedClientsLeft)
	require.EqualValues(t, 10, haStatus.PrimaryServer.AnalyzedPackets)

	require.Equal(t, "secondary", haStatus.SecondaryServer.Role)
	require.Len(t, haStatus.SecondaryServer.Scopes, 1)
	require.Contains(t, haStatus.SecondaryServer.Scopes, "server2")
	require.Equal(t, "load-balancing", haStatus.SecondaryServer.State)
	require.GreaterOrEqual(t, haStatus.SecondaryServer.Age, int64(5))
	require.False(t, haStatus.SecondaryServer.InTouch)
	require.Empty(t, haStatus.SecondaryServer.ControlAddress)
	require.NotEmpty(t, haStatus.SecondaryServer.StatusTime.String())
	require.EqualValues(t, 1, haStatus.SecondaryServer.CommInterrupted)
	require.EqualValues(t, 9, haStatus.SecondaryServer.ConnectingClients)
	require.EqualValues(t, 4, haStatus.SecondaryServer.UnackedClients)
	require.EqualValues(t, 9, haStatus.SecondaryServer.UnackedClientsLeft)
	require.EqualValues(t, 11, haStatus.SecondaryServer.AnalyzedPackets)

	// Validate the status of the DHCPv6 pair.
	status = statusList[1].Status.KeaStatus
	require.NotNil(t, status.HaServers)

	haStatus = status.HaServers
	require.NotNil(t, haStatus.PrimaryServer)
	require.NotNil(t, haStatus.SecondaryServer)

	require.EqualValues(t, keaApp.Daemons[0].ID, haStatus.PrimaryServer.ID)
	require.Equal(t, "primary", haStatus.PrimaryServer.Role)
	require.Len(t, haStatus.PrimaryServer.Scopes, 1)
	require.Contains(t, haStatus.PrimaryServer.Scopes, "server1")
	require.Equal(t, "hot-standby", haStatus.PrimaryServer.State)
	require.GreaterOrEqual(t, haStatus.PrimaryServer.Age, int64(5))
	require.Equal(t, "127.0.0.1", haStatus.PrimaryServer.ControlAddress)
	require.EqualValues(t, 1, haStatus.PrimaryServer.CommInterrupted)
	require.EqualValues(t, 5, haStatus.PrimaryServer.ConnectingClients)
	require.EqualValues(t, 2, haStatus.PrimaryServer.UnackedClients)
	require.EqualValues(t, 9, haStatus.PrimaryServer.UnackedClientsLeft)
	require.EqualValues(t, 123, haStatus.PrimaryServer.AnalyzedPackets)

	require.Equal(t, "standby", haStatus.SecondaryServer.Role)
	require.Empty(t, haStatus.SecondaryServer.Scopes)
	require.Equal(t, "waiting", haStatus.SecondaryServer.State)
	require.GreaterOrEqual(t, haStatus.SecondaryServer.Age, int64(5))
	require.False(t, haStatus.SecondaryServer.InTouch)
	require.Empty(t, haStatus.SecondaryServer.ControlAddress)
	require.EqualValues(t, -1, haStatus.SecondaryServer.CommInterrupted)
	require.Zero(t, haStatus.SecondaryServer.ConnectingClients)
	require.Zero(t, haStatus.SecondaryServer.UnackedClients)
	require.Zero(t, haStatus.SecondaryServer.UnackedClientsLeft)
	require.Zero(t, haStatus.SecondaryServer.AnalyzedPackets)
}

// Test that status of a HA service providing passive-backup mode is
// parsed correctly.
func TestRestGetAppServicesStatusPassiveBackup(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	// Configure the fake control agents to mimic returning a status of
	// two HA services for Kea.
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// Add a machine.
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// Add Kea application to the machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "127.0.0.1", "", 1234, true)
	keaApp := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: keaPoints,
		Daemons: []*dbmodel.Daemon{
			dbmodel.NewKeaDaemon("dhcp4", true),
		},
	}
	_, err = dbmodel.AddApp(db, keaApp)
	require.NoError(t, err)
	require.NotZero(t, keaApp.ID)

	exampleTime := storkutil.UTCNow().Add(-5 * time.Second)
	keaServices := []dbmodel.Service{
		{
			BaseService: dbmodel.BaseService{
				ServiceType: "ha_dhcp",
			},
			HAService: &dbmodel.BaseHAService{
				HAType:                   "dhcp4",
				HAMode:                   "passive-backup",
				PrimaryID:                keaApp.ID,
				PrimaryStatusCollectedAt: exampleTime,
				PrimaryLastState:         "passive-backup",
				PrimaryLastScopes:        []string{"server1"},
			},
		},
	}

	// Add the services and associate them with the app.
	for i := range keaServices {
		err = dbmodel.AddService(db, &keaServices[i])
		require.NoError(t, err)
		err = dbmodel.AddDaemonToService(db, keaServices[i].ID, keaApp.Daemons[0])
		require.NoError(t, err)
	}

	params := services.GetAppServicesStatusParams{
		ID: keaApp.ID,
	}
	rsp := rapi.GetAppServicesStatus(ctx, params)

	// Make sure that the response is ok.
	require.IsType(t, &services.GetAppServicesStatusOK{}, rsp)
	okRsp := rsp.(*services.GetAppServicesStatusOK)
	require.NotNil(t, okRsp.Payload.Items)

	// There should be one structure returned with a status of the
	// DHCPv4 server.
	require.Len(t, okRsp.Payload.Items, 1)

	statusList := okRsp.Payload.Items

	// Validate the status of the DHCPv4 pair.
	status := statusList[0].Status.KeaStatus
	require.NotNil(t, status.HaServers)

	haStatus := status.HaServers
	require.NotNil(t, haStatus.PrimaryServer)
	require.Nil(t, haStatus.SecondaryServer)

	require.EqualValues(t, keaApp.Daemons[0].ID, haStatus.PrimaryServer.ID)
	require.Equal(t, "primary", haStatus.PrimaryServer.Role)
	require.Len(t, haStatus.PrimaryServer.Scopes, 1)
	require.Contains(t, haStatus.PrimaryServer.Scopes, "server1")
	require.Equal(t, "passive-backup", haStatus.PrimaryServer.State)
	require.GreaterOrEqual(t, haStatus.PrimaryServer.Age, int64(5))
	require.Equal(t, "127.0.0.1", haStatus.PrimaryServer.ControlAddress)
	require.EqualValues(t, keaApp.ID, haStatus.PrimaryServer.AppID)
	require.NotEmpty(t, haStatus.PrimaryServer.StatusTime.String())
	require.EqualValues(t, -1, haStatus.PrimaryServer.CommInterrupted)
	require.Zero(t, haStatus.PrimaryServer.ConnectingClients)
	require.Zero(t, haStatus.PrimaryServer.UnackedClients)
	require.Zero(t, haStatus.PrimaryServer.UnackedClientsLeft)
	require.Zero(t, haStatus.PrimaryServer.AnalyzedPackets)
}

func TestRestGetAppsStats(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// get empty list of app
	params := services.GetAppsStatsParams{}
	rsp := rapi.GetAppsStats(ctx, params)
	require.IsType(t, &services.GetAppsStatsOK{}, rsp)
	okRsp := rsp.(*services.GetAppsStatsOK)
	require.Zero(t, okRsp.Payload.KeaAppsTotal)
	require.Zero(t, okRsp.Payload.KeaAppsNotOk)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// add app kea to machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "", "", 1234, false)
	s1 := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: keaPoints,
		Daemons:      []*dbmodel.Daemon{},
	}
	_, err = dbmodel.AddApp(db, s1)
	require.NoError(t, err)

	// add app bind9 to machine
	var bind9Points []*dbmodel.AccessPoint
	bind9Points = dbmodel.AppendAccessPoint(bind9Points, dbmodel.AccessPointControl, "", "abcd", 4321, true)
	s2 := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeBind9,
		Active:       false,
		AccessPoints: bind9Points,
		Daemons:      []*dbmodel.Daemon{},
	}
	_, err = dbmodel.AddApp(db, s2)
	require.NoError(t, err)

	// get added app
	params = services.GetAppsStatsParams{}
	rsp = rapi.GetAppsStats(ctx, params)
	require.IsType(t, &services.GetAppsStatsOK{}, rsp)
	okRsp = rsp.(*services.GetAppsStatsOK)
	require.EqualValues(t, 1, okRsp.Payload.KeaAppsTotal)
	require.Zero(t, okRsp.Payload.KeaAppsNotOk)
	require.EqualValues(t, 1, okRsp.Payload.Bind9AppsTotal)
	require.EqualValues(t, 1, okRsp.Payload.Bind9AppsNotOk)
}

func TestGetDhcpOverview(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// add app kea to machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "localhost", "", 1234, false)
	app := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Name:         "test-app",
		Active:       true,
		AccessPoints: keaPoints,
		Daemons: []*dbmodel.Daemon{
			dbmodel.NewKeaDaemon("dhcp4", true),
			dbmodel.NewKeaDaemon("dhcp6", true),
		},
	}
	// dhcp6 is not monitored, only dhcp4 should be visible
	app.Daemons[1].Monitored = false
	_, err = dbmodel.AddApp(db, app)
	require.NoError(t, err)

	err = dbmodel.AddSubnet(db, &dbmodel.Subnet{
		Prefix: "3001:fed8::/64",
		LocalSubnets: []*dbmodel.LocalSubnet{
			{
				Stats: nil,
			},
			{
				Stats: dbmodel.SubnetStats{
					"total-addresses": 0,
				},
			},
		},
	})
	require.NoError(t, err)

	// get overview, generally it should be empty
	params := dhcp.GetDhcpOverviewParams{}
	rsp := rapi.GetDhcpOverview(ctx, params)
	require.IsType(t, &dhcp.GetDhcpOverviewOK{}, rsp)
	okRsp := rsp.(*dhcp.GetDhcpOverviewOK)
	require.Len(t, okRsp.Payload.Subnets4.Items, 0)
	require.Len(t, okRsp.Payload.Subnets6.Items, 1)
	require.Nil(t, okRsp.Payload.Subnets6.Items[0].LocalSubnets)
	require.Len(t, okRsp.Payload.SharedNetworks4.Items, 0)
	require.Len(t, okRsp.Payload.SharedNetworks6.Items, 0)
	require.Len(t, okRsp.Payload.DhcpDaemons, 1)

	// only dhcp4 is present
	require.EqualValues(t, "dhcp4", okRsp.Payload.DhcpDaemons[0].Name)
	require.EqualValues(t, "test-app", okRsp.Payload.DhcpDaemons[0].AppName)
	require.EqualValues(t, 1, okRsp.Payload.DhcpDaemons[0].AgentCommErrors)
	require.EqualValues(t, 2, okRsp.Payload.DhcpDaemons[0].CaCommErrors)
	require.EqualValues(t, 5, okRsp.Payload.DhcpDaemons[0].DaemonCommErrors)

	// HA is not enabled.
	require.False(t, okRsp.Payload.DhcpDaemons[0].HaEnabled)
	require.Empty(t, okRsp.Payload.DhcpDaemons[0].HaState)
}

// This test verifies that the overview response includes HA state.
func TestHAInDhcpOverview(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	// Configure the fake control agents to mimic returning a status of
	// two HA services for Kea.
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// Add a machine.
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// Add Kea application to the machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "127.0.0.1", "", 1234, true)
	keaApp := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: keaPoints,
		Daemons: []*dbmodel.Daemon{
			dbmodel.NewKeaDaemon("dhcp4", true),
		},
	}
	_, err = dbmodel.AddApp(db, keaApp)
	require.NoError(t, err)
	require.NotZero(t, keaApp.ID)

	// Create an HA service.
	exampleTime := storkutil.UTCNow().Add(-5 * time.Second)
	keaService := dbmodel.Service{
		BaseService: dbmodel.BaseService{
			ServiceType: "ha_dhcp",
		},
		HAService: &dbmodel.BaseHAService{
			HAType:                   "dhcp4",
			HAMode:                   "load-balancing",
			PrimaryID:                keaApp.ID,
			PrimaryStatusCollectedAt: exampleTime,
			PrimaryLastState:         "load-balancing",
			SecondaryLastFailoverAt:  exampleTime,
		},
	}

	// Add the service and associate the daemon with that service.
	err = dbmodel.AddService(db, &keaService)
	require.NoError(t, err)
	err = dbmodel.AddDaemonToService(db, keaService.ID, keaApp.Daemons[0])
	require.NoError(t, err)

	// Get the overview.
	params := dhcp.GetDhcpOverviewParams{}
	rsp := rapi.GetDhcpOverview(ctx, params)
	require.IsType(t, &dhcp.GetDhcpOverviewOK{}, rsp)
	okRsp := rsp.(*dhcp.GetDhcpOverviewOK)
	require.Len(t, okRsp.Payload.DhcpDaemons, 1)

	// Test that the HA specific information was returned.
	require.True(t, okRsp.Payload.DhcpDaemons[0].HaEnabled)
	require.Equal(t, "load-balancing", okRsp.Payload.DhcpDaemons[0].HaState)
	require.NotEmpty(t, okRsp.Payload.DhcpDaemons[0].HaFailureAt.String())
}

// Test that the DHCP Overview is properly generated when the statistic
// table contains the NULL values.
func TestGetDhcpOverviewWithNullStatistics(t *testing.T) {
	// Arrange
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	_ = dbmodel.InitializeStats(db)

	rapi, _ := NewRestAPI(db, dbSettings)
	ctx := context.Background()
	params := dhcp.GetDhcpOverviewParams{}

	// Act
	err := dbmodel.SetStats(db, map[string]*big.Int{"total-addresses": nil})
	rsp := rapi.GetDhcpOverview(ctx, params)

	// Assert
	require.NoError(t, err)
	require.IsType(t, &dhcp.GetDhcpOverviewOK{}, rsp)
	okRsp := rsp.(*dhcp.GetDhcpOverviewOK)
	require.EqualValues(t, "<nil>", okRsp.Payload.Dhcp4Stats.TotalAddresses)
}

// Test updating daemon.
func TestUpdateDaemon(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	// Configure the fake control agents to mimic returning a status of
	// two HA services for Kea.
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// Add a machine.
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// Add Kea application to the machine
	var keaPoints []*dbmodel.AccessPoint
	keaPoints = dbmodel.AppendAccessPoint(keaPoints, dbmodel.AccessPointControl, "127.0.0.1", "", 1234, false)
	keaApp := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: keaPoints,
		Daemons: []*dbmodel.Daemon{
			dbmodel.NewKeaDaemon("dhcp4", true),
		},
	}
	_, err = dbmodel.AddApp(db, keaApp)
	require.NoError(t, err)
	require.NotZero(t, keaApp.ID)
	require.NotZero(t, keaApp.Daemons[0].ID)

	// get added app
	getAppParams := services.GetAppParams{
		ID: keaApp.ID,
	}
	rsp := rapi.GetApp(ctx, getAppParams)
	require.IsType(t, &services.GetAppOK{}, rsp)
	okRsp := rsp.(*services.GetAppOK)
	require.Equal(t, keaApp.ID, okRsp.Payload.ID)
	require.True(t, okRsp.Payload.Details.AppKea.Daemons[0].Monitored) // now it is true

	// setup a user session (UpdateDaemon needs user db object)
	user, err := dbmodel.GetUserByID(rapi.DB, 1)
	require.NoError(t, err)
	ctx2, err := rapi.SessionManager.Load(ctx, "")
	require.NoError(t, err)
	err = rapi.SessionManager.LoginHandler(ctx2, user)
	require.NoError(t, err)

	// update daemon: change monitored to false
	params := services.UpdateDaemonParams{
		ID: keaApp.Daemons[0].ID,
		Daemon: services.UpdateDaemonBody{
			Monitored: false,
		},
	}
	rsp = rapi.UpdateDaemon(ctx2, params)
	require.IsType(t, &services.UpdateDaemonOK{}, rsp)

	// get app with modified daemon
	rsp = rapi.GetApp(ctx2, getAppParams)
	require.IsType(t, &services.GetAppOK{}, rsp)
	okRsp = rsp.(*services.GetAppOK)
	require.Equal(t, keaApp.ID, okRsp.Payload.ID)
	require.False(t, okRsp.Payload.Details.AppKea.Daemons[0].Monitored) // now it is false
}

// Check if generating and getting server token works.
func TestServerToken(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()
	settings := RestAPISettings{}

	// Configure the fake control and prepare rest API.
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// setup a user session, it is required to check user role in GetMachinesServerToken
	user, err := dbmodel.GetUserByID(rapi.DB, 1)
	require.NoError(t, err)
	ctx2, err := rapi.SessionManager.Load(ctx, "")
	require.NoError(t, err)
	err = rapi.SessionManager.LoginHandler(ctx2, user)
	require.NoError(t, err)

	// get server token but it was not created yet, so error is expected
	params := services.GetMachinesServerTokenParams{}
	rsp := rapi.GetMachinesServerToken(ctx2, params)
	require.IsType(t, &services.GetMachinesServerTokenDefault{}, rsp)
	errRsp := rsp.(*services.GetMachinesServerTokenDefault)
	require.EqualValues(t, "Server internal problem - server token is empty", *errRsp.Payload.Message)

	// generate server token
	serverToken, err := certs.GenerateServerToken(db)
	require.NoError(t, err)
	require.NotEmpty(t, serverToken)

	// get server token
	rsp = rapi.GetMachinesServerToken(ctx2, params)
	require.IsType(t, &services.GetMachinesServerTokenOK{}, rsp)
	okRsp := rsp.(*services.GetMachinesServerTokenOK)
	require.EqualValues(t, serverToken, okRsp.Payload.Token)

	// regenerate server token
	regenParams := services.RegenerateMachinesServerTokenParams{}
	rsp = rapi.RegenerateMachinesServerToken(ctx2, regenParams)
	require.IsType(t, &services.RegenerateMachinesServerTokenOK{}, rsp)
	okRegenRsp := rsp.(*services.RegenerateMachinesServerTokenOK)
	require.NotEmpty(t, okRegenRsp.Payload.Token)
}

// Test that a PUT request to rename an app will cause the app to be successfully renamed.
// Also, test that invalid app name will cause an error.
func TestRenameApp(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	// Add a machine.
	machine := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err := dbmodel.AddMachine(db, machine)
	require.NoError(t, err)

	// Add Kea application to the machine
	app := &dbmodel.App{
		MachineID: machine.ID,
		Type:      dbmodel.AppTypeKea,
		Name:      "dhcp-server1",
	}
	_, err = dbmodel.AddApp(db, app)
	require.NoError(t, err)
	require.NotZero(t, app.ID)

	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	require.NoError(t, err)
	ctx := context.Background()

	// Use correct parameters.
	newName := "dhcp-server2"
	params := services.RenameAppParams{
		ID: app.ID,
		NewAppName: services.RenameAppBody{
			Name: &newName,
		},
	}
	rsp := rapi.RenameApp(ctx, params)
	require.IsType(t, &services.RenameAppOK{}, rsp)

	// Make sure the app has been successfully renamed.
	returnedApp, err := dbmodel.GetAppByID(db, app.ID)
	require.NoError(t, err)
	require.NotNil(t, returnedApp)
	require.Equal(t, "dhcp-server2", returnedApp.Name)

	// Use an incorrect app name.
	newName = "dhcp-server2@machine3"
	rsp = rapi.RenameApp(ctx, params)
	require.IsType(t, &services.RenameAppDefault{}, rsp)
	defaultRsp := rsp.(*services.RenameAppDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))

	// Ensure that the event informing about renaming the app was emitted.
	require.Len(t, fec.Events, 1)
	require.Contains(t, fec.Events[0].Text, "renamed from dhcp-server1")
	require.NotNil(t, fec.Events[0].Relations)
	require.Equal(t, machine.ID, fec.Events[0].Relations.MachineID)
	require.Equal(t, app.ID, fec.Events[0].Relations.AppID)

	// Empty name (with only whitespace) should cause an error too.
	newName = "   "
	rsp = rapi.RenameApp(ctx, params)
	require.IsType(t, &services.RenameAppDefault{}, rsp)
	defaultRsp = rsp.(*services.RenameAppDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))

	// Finally, let's try supplying a nil value.
	params.NewAppName = services.RenameAppBody{
		Name: nil,
	}
	rsp = rapi.RenameApp(ctx, params)
	require.IsType(t, &services.RenameAppDefault{}, rsp)
	defaultRsp = rsp.(*services.RenameAppDefault)
	require.Equal(t, http.StatusBadRequest, getStatusCode(*defaultRsp))
}

// This test verifies that database backend configurations are parsed correctly
// from the Kea configuration and formatted such that they can be returned over
// the REST API.
func TestGetKeaStorages(t *testing.T) {
	configString := `{
        "Dhcp4": {
            "lease-database": {
                "type": "memfile",
                "name": "/tmp/leases4.csv"
            },
            "hosts-database": {
                "type": "mysql",
                "name": "kea-hosts-mysql",
                "host": "mysql.example.org"
            },
            "config-control": {
                "config-databases": [
                    {
                        "type": "mysql",
                        "name": "kea-config-mysql"
                    }
                ]
            },
            "hooks-libraries": [
                {
                    "library": "/usr/lib/kea/libdhcp_legal_log.so",
                    "parameters": {
                         "path": "/tmp/legal_log.log"
                    }
                }
            ]
        }
    }`
	keaConfig, err := keaconfig.NewConfig(configString)
	require.NoError(t, err)
	require.NotNil(t, keaConfig)

	files, databases := getKeaStorages(keaConfig)
	require.Len(t, files, 2)
	require.Len(t, databases, 2)

	// Ensure that the lease file is present.
	require.Equal(t, "/tmp/leases4.csv", files[0].Filename)
	require.Equal(t, "Lease file", files[0].Filetype)

	// Ensure that the forensic logging file is present.
	require.Equal(t, "/tmp/legal_log.log", files[1].Filename)
	require.Equal(t, "Forensic Logging", files[1].Filetype)

	// Test database configurations.
	for _, d := range databases {
		if d.Database == "kea-hosts-mysql" {
			require.Equal(t, "mysql", d.BackendType)
			require.Equal(t, "mysql.example.org", d.Host)
		} else {
			require.Equal(t, "mysql", d.BackendType)
			require.Equal(t, "kea-config-mysql", d.Database)
			require.Equal(t, "localhost", d.Host)
		}
	}
}

// Test that converting app with nil Kea config doesn't cause panic.
func TestAppToRestAPIForNilKeaConfig(t *testing.T) {
	// Arrange
	app := &dbmodel.App{
		MachineID: 1,
		Type:      dbmodel.AppTypeKea,
		Daemons: []*dbmodel.Daemon{
			dbmodel.NewKeaDaemon("dhcp4", true),
		},
	}
	rapi, err := NewRestAPI(&dbops.DatabaseSettings{})
	require.NoError(t, err)

	// Act
	restApp := rapi.appToRestAPI(app)

	// Assert
	require.NotNil(t, restApp)
}

// Test conversion of a KeaDaemon to REST API format.
func TestKeaDaemonToRestAPI(t *testing.T) {
	daemon := &dbmodel.Daemon{
		ID:              1,
		Pid:             1234,
		Name:            "dhcp4",
		Active:          true,
		Monitored:       true,
		Version:         "2.1",
		ExtendedVersion: "2.1.x",
		Uptime:          1000,
		ReloadedAt:      time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
		KeaDaemon: &dbmodel.KeaDaemon{
			Config: dbmodel.NewKeaConfig(&map[string]interface{}{
				"Dhcp4": map[string]interface{}{
					"hooks-libraries": []interface{}{
						map[string]interface{}{
							"library": "hook_abc.so",
						},
						map[string]interface{}{
							"library": "hook_def.so",
						},
					},
				},
			}),
		},
		App: &dbmodel.App{
			ID:   2,
			Name: "funny",
		},
	}
	converted := keaDaemonToRestAPI(daemon)
	require.NotNil(t, converted)
	require.EqualValues(t, daemon.ID, converted.ID)
	require.EqualValues(t, daemon.Pid, converted.Pid)
	require.Equal(t, daemon.Active, converted.Active)
	require.Equal(t, daemon.Monitored, converted.Monitored)
	require.Equal(t, daemon.Version, converted.Version)
	require.Equal(t, daemon.ExtendedVersion, converted.ExtendedVersion)
	require.EqualValues(t, daemon.Uptime, converted.Uptime)
	require.Equal(t, "2009-11-10T23:00:00.000Z", converted.ReloadedAt.String())
	require.Len(t, converted.Hooks, 2)
	require.Contains(t, converted.Hooks, "hook_abc.so")
	require.Contains(t, converted.Hooks, "hook_def.so")
	require.NotNil(t, converted.App)
	require.EqualValues(t, daemon.App.ID, converted.App.ID)
	require.Equal(t, daemon.App.Name, converted.App.Name)
}

// This test verifies that the lease database configuration storing the
// leases in an SQL database is correctly recognized.
func TestGetKeaStoragesLeaseDatabase(t *testing.T) {
	configString := `{
        "Dhcp4": {
            "lease-database": {
                "type": "postgresql",
                "name": "kea"
            }
        }
    }`
	keaConfig, err := keaconfig.NewConfig(configString)
	require.NoError(t, err)
	require.NotNil(t, keaConfig)

	files, databases := getKeaStorages(keaConfig)
	require.Empty(t, files, 0)
	require.Len(t, databases, 1)

	require.Equal(t, "postgresql", databases[0].BackendType)
	require.Equal(t, "kea", databases[0].Database)
	require.Equal(t, "localhost", databases[0].Host)
}

// This test verifies that the forensic logging database configuration is
// correctly recognized.
func TestGetKeaStoragesForensicDatabase(t *testing.T) {
	configString := `{
        "Dhcp4": {
            "hooks-libraries": [
                {
                    "library": "/usr/lib/kea/libdhcp_legal_log.so",
                    "parameters": {
                         "type": "mysql",
                         "name": "kea"
                    }
                }
            ]
        }
    }`
	keaConfig, err := keaconfig.NewConfig(configString)
	require.NoError(t, err)
	require.NotNil(t, keaConfig)

	files, databases := getKeaStorages(keaConfig)
	require.Empty(t, files, 0)
	require.Len(t, databases, 1)

	require.Equal(t, "mysql", databases[0].BackendType)
	require.Equal(t, "kea", databases[0].Database)
	require.Equal(t, "localhost", databases[0].Host)
}

// Test that the GetMachineDump returns OK status when the machine exists.
func TestGetMachineDumpOK(t *testing.T) {
	// Arrange
	// Database init
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()
	_ = dbmodel.InitializeSettings(db, 0)
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	_ = dbmodel.AddMachine(db, m)
	// REST init
	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, _ := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	ctx := context.Background()
	// User init
	user, _ := dbmodel.GetUserByID(rapi.DB, 1)
	ctx, _ = rapi.SessionManager.Load(ctx, "")
	_ = rapi.SessionManager.LoginHandler(ctx, user)
	// Request init
	params := services.GetMachineDumpParams{
		ID: m.ID,
	}

	// Act
	rsp := rapi.GetMachineDump(ctx, params)

	// Assert
	require.IsType(t, &services.GetMachineDumpOK{}, rsp)
}

// Test that the GetMachineDump returns a tarball file.
func TestGetMachineDumpReturnsTarball(t *testing.T) {
	// Arrange
	// Database init
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()
	_ = dbmodel.InitializeSettings(db, 0)
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	_ = dbmodel.AddMachine(db, m)
	// REST init
	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, _ := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	ctx := context.Background()
	// User init
	user, _ := dbmodel.GetUserByID(rapi.DB, 1)
	ctx, _ = rapi.SessionManager.Load(ctx, "")
	_ = rapi.SessionManager.LoginHandler(ctx, user)
	// Request init
	params := services.GetMachineDumpParams{
		ID: m.ID,
	}

	// Act
	rsp := rapi.GetMachineDump(ctx, params).(*services.GetMachineDumpOK)
	filenames, err := storkutil.ListFilesInTarball(rsp.Payload)

	// Assert
	require.NoError(t, err)
	require.NotZero(t, len(filenames))
}

// Test that the GetMachineDump returns HTTP 404 Not Found when the machine is missing.
func TestGetMachineDumpNotExists(t *testing.T) {
	// Arrange
	// Database init
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()
	_ = dbmodel.InitializeSettings(db, 0)
	// REST init
	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, _ := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	ctx := context.Background()
	// User init
	user, _ := dbmodel.GetUserByID(rapi.DB, 1)
	ctx, _ = rapi.SessionManager.Load(ctx, "")
	_ = rapi.SessionManager.LoginHandler(ctx, user)
	// Request init
	params := services.GetMachineDumpParams{
		ID: 42,
	}

	// Act
	rsp := rapi.GetMachineDump(ctx, params)

	// Assert
	defaultRsp, ok := rsp.(*services.GetMachineDumpDefault)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
}

// Test that the GetMachineDump returns a file with expected filename.
func TestGetMachineDumpReturnsExpectedFilename(t *testing.T) {
	// Arrange
	// Database init
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()
	_ = dbmodel.InitializeSettings(db, 0)
	m := &dbmodel.Machine{
		ID:        42,
		Address:   "localhost",
		AgentPort: 8080,
	}
	_ = dbmodel.AddMachine(db, m)
	// REST init
	settings := RestAPISettings{}
	fa := agentcommtest.NewFakeAgents(nil, nil)
	fec := &storktest.FakeEventCenter{}
	fd := &storktest.FakeDispatcher{}
	rapi, _ := NewRestAPI(&settings, dbSettings, db, fa, fec, fd)
	ctx := context.Background()
	// User init
	user, _ := dbmodel.GetUserByID(rapi.DB, 1)
	ctx, _ = rapi.SessionManager.Load(ctx, "")
	_ = rapi.SessionManager.LoginHandler(ctx, user)
	// Request init
	params := services.GetMachineDumpParams{
		ID: m.ID,
	}

	headerPattern := regexp.MustCompile("attachment; filename=\"(.*)\"")

	// Act
	rsp := rapi.GetMachineDump(ctx, params).(*services.GetMachineDumpOK)
	headerValue := rsp.ContentDisposition
	submatches := headerPattern.FindStringSubmatch(headerValue)

	// Assert
	require.Len(t, submatches, 2)
	filename := submatches[1]
	rest, timestamp, extension, err := testutil.ParseTimestampFilename(filename)
	require.NoError(t, err)
	require.Contains(t, rest, "machine-42")
	require.EqualValues(t, extension, ".tar.gz")
	require.LessOrEqual(t, time.Now().UTC().Sub(timestamp).Seconds(), float64(10))
}

// Test that only super-administrators can fetch the authentication key of the
// access point.
func TestGetAccessPointKeyIsRestrictedToSuperAdmins(t *testing.T) {
	// Arrange
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := &RestAPISettings{}
	rapi, _ := NewRestAPI(settings, dbSettings, db)

	// Create an admin user.
	ctx, _ := rapi.SessionManager.Load(context.Background(), "")
	user := &dbmodel.SystemUser{
		Login:    "foo",
		Name:     "baz",
		Lastname: "boz",
		Groups:   []*dbmodel.SystemGroup{{ID: dbmodel.AdminGroupID}},
	}
	_, _ = dbmodel.CreateUser(rapi.DB, user)
	_ = rapi.SessionManager.LoginHandler(ctx, user)

	// Act
	rsp := rapi.GetAccessPointKey(ctx, services.GetAccessPointKeyParams{
		AppID: 42,
		Type:  dbmodel.AccessPointControl,
	})

	// Assert
	defaultRsp, ok := rsp.(*services.GetAccessPointKeyDefault)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, getStatusCode(*defaultRsp))
}

// Test that the HTTP 500 Internal Server Error status is returned if the
// database is not available.
func TestGetAccessPointKeyForInvalidDatabase(t *testing.T) {
	// Arrange
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)

	settings := &RestAPISettings{}
	rapi, _ := NewRestAPI(settings, dbSettings, db)

	ctx, _ := rapi.SessionManager.Load(context.Background(), "")
	user, _ := dbmodel.GetUserByID(rapi.DB, 1)
	_ = rapi.SessionManager.LoginHandler(ctx, user)

	// Act
	teardown()
	rsp := rapi.GetAccessPointKey(ctx, services.GetAccessPointKeyParams{
		AppID: 42,
		Type:  dbmodel.AccessPointControl,
	})

	// Assert
	defaultRsp, ok := rsp.(*services.GetAccessPointKeyDefault)
	require.True(t, ok)
	require.Equal(t, http.StatusInternalServerError, getStatusCode(*defaultRsp))
}

// Test that the HTTP 404 Not Found status is returned if the access point
// doesn't exist.
func TestGetAccessPointKeyForMissingEntry(t *testing.T) {
	// Arrange
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := &RestAPISettings{}
	rapi, _ := NewRestAPI(settings, dbSettings, db)

	ctx, _ := rapi.SessionManager.Load(context.Background(), "")
	user, _ := dbmodel.GetUserByID(rapi.DB, 1)
	_ = rapi.SessionManager.LoginHandler(ctx, user)

	// Act
	rsp := rapi.GetAccessPointKey(ctx, services.GetAccessPointKeyParams{
		AppID: 42,
		Type:  dbmodel.AccessPointControl,
	})

	// Assert
	defaultRsp, ok := rsp.(*services.GetAccessPointKeyDefault)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, getStatusCode(*defaultRsp))
}

// Test that the authentication key of a given access point is fetched properly.
func TestGetAccessPointKey(t *testing.T) {
	// Arrange
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := &RestAPISettings{}
	rapi, _ := NewRestAPI(settings, dbSettings, db)

	ctx, _ := rapi.SessionManager.Load(context.Background(), "")
	user, _ := dbmodel.GetUserByID(rapi.DB, 1)
	_ = rapi.SessionManager.LoginHandler(ctx, user)

	machine := &dbmodel.Machine{Address: "localhost", AgentPort: 8080}
	_ = dbmodel.AddMachine(db, machine)
	app := &dbmodel.App{
		MachineID: machine.ID,
		Type:      dbmodel.AppTypeBind9,
		AccessPoints: []*dbmodel.AccessPoint{{
			Type:              dbmodel.AccessPointControl,
			Address:           "127.0.0.1",
			Port:              8080,
			Key:               "secret",
			UseSecureProtocol: true,
		}},
	}
	_, _ = dbmodel.AddApp(db, app)

	// Act
	rsp := rapi.GetAccessPointKey(ctx, services.GetAccessPointKeyParams{
		AppID: app.ID,
		Type:  dbmodel.AccessPointControl,
	})

	// Assert
	okRsp, ok := rsp.(*services.GetAccessPointKeyOK)
	require.True(t, ok)
	require.EqualValues(t, "secret", okRsp.Payload)
}
