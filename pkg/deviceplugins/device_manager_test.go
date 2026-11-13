package deviceplugins

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	pluginapi "kubevirt.io/kubevirt/pkg/virt-handler/device-manager/deviceplugin/v1beta1"
)

// fakeKubelet implements the kubelet registration service. Like kubelet, it
// opens a ListAndWatch stream to every plugin that registers.
type fakeKubelet struct {
	dir           string
	registrations atomic.Int32
	streams       atomic.Int32
}

func (k *fakeKubelet) Register(_ context.Context, req *pluginapi.RegisterRequest) (*pluginapi.Empty, error) {
	k.registrations.Add(1)
	go k.listAndWatch(filepath.Join(k.dir, req.Endpoint))
	return &pluginapi.Empty{}, nil
}

func (k *fakeKubelet) listAndWatch(socket string) {
	conn, err := gRPCConnect(context.Background(), socket, 5*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	stream, err := pluginapi.NewDevicePluginClient(conn).ListAndWatch(context.Background(), &pluginapi.Empty{})
	if err != nil {
		return
	}
	if _, err := stream.Recv(); err != nil {
		return
	}
	k.streams.Add(1)
	for {
		if _, err := stream.Recv(); err != nil {
			return
		}
	}
}

func startFakeKubelet(t *testing.T, dir string) *fakeKubelet {
	t.Helper()

	socket := filepath.Join(dir, "kubelet.sock")
	oldKubeletSocket := kubeletSocket
	kubeletSocket = socket
	t.Cleanup(func() { kubeletSocket = oldKubeletSocket })

	lis, err := net.Listen("unix", socket)
	require.NoError(t, err)

	kubelet := &fakeKubelet{dir: dir}
	server := grpc.NewServer()
	pluginapi.RegisterRegistrationServer(server, kubelet)
	go server.Serve(lis) //nolint:errcheck
	t.Cleanup(server.Stop)

	return kubelet
}

// Removing the plugin socket is what kubelet does when it restarts. The plugin
// must serve a new socket and register again, and Stop must end the loop.
func TestPCIDevicePluginReRegistersAfterSocketRemoval(t *testing.T) {
	dir := t.TempDir()
	// healthCheck watches <deviceRoot>/dev, so it must exist.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dev"), 0o755))

	kubelet := startFakeKubelet(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dp := NewPCIDevicePlugin(ctx, nil, "example.com/fake-device")
	dp.socketPath = filepath.Join(dir, "kubevirt-example.com-fake-device.sock")
	dp.deviceRoot = dir
	dp.starter.backoff = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}

	stop := make(chan struct{})
	startErr := make(chan error, 1)
	go func() { startErr <- dp.Start(stop) }()
	dp.SetStarted(stop)

	require.Eventually(t, func() bool { return kubelet.streams.Load() == 1 },
		10*time.Second, 10*time.Millisecond, "plugin did not register with kubelet")
	// Give healthCheck time to set up its socket watch.
	time.Sleep(200 * time.Millisecond)

	// Simulate a kubelet restart.
	require.NoError(t, os.Remove(dp.socketPath))

	require.Eventually(t, func() bool {
		return kubelet.registrations.Load() == 2 && kubelet.streams.Load() == 2
	}, 10*time.Second, 10*time.Millisecond, "plugin did not register again after its socket was removed")
	_, err := os.Stat(dp.socketPath)
	require.NoError(t, err, "plugin socket was not created again")

	// Stop must end Start and not restart the plugin.
	require.NoError(t, dp.Stop())
	select {
	case err := <-startErr:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return after Stop")
	}
	_, err = os.Stat(dp.socketPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Equal(t, int32(2), kubelet.registrations.Load())
}
