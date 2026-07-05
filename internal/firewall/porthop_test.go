package firewall

import (
	"errors"
	"testing"
)

type fakePortHopBackend struct {
	forwardErr        error
	removeForwardErr  error
	removeRangeErr    error
	addRangeCalled    bool
	removeRangeCalled bool
}

func (f *fakePortHopBackend) AddPort(port int, protocol string) error { return nil }
func (f *fakePortHopBackend) RemovePort(port int, protocol string) error {
	return nil
}
func (f *fakePortHopBackend) AddPortRange(start, end int, protocol string) error {
	f.addRangeCalled = true
	return nil
}
func (f *fakePortHopBackend) RemovePortRange(start, end int, protocol string) error {
	f.removeRangeCalled = true
	return f.removeRangeErr
}
func (f *fakePortHopBackend) AddPortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	return f.forwardErr
}
func (f *fakePortHopBackend) RemovePortForward(srcStart, srcEnd, dstPort int, protocol string) error {
	return f.removeForwardErr
}
func (f *fakePortHopBackend) IsActive() bool { return true }

func TestSetupPortHoppingRequiresFirewallBackend(t *testing.T) {
	m := &Manager{}
	err := m.SetupPortHopping(&PortHopConfig{
		StartPort:  30000,
		EndPort:    30010,
		TargetPort: 10080,
		Protocol:   "udp",
	})
	if err == nil {
		t.Fatal("expected port hopping to fail without a firewall backend")
	}
}

func TestRemovePortHoppingReturnsFirewallErrors(t *testing.T) {
	backend := &fakePortHopBackend{
		removeForwardErr: errors.New("remove forward failed"),
		removeRangeErr:   errors.New("remove range failed"),
	}
	m := &Manager{backend: backend}

	err := m.RemovePortHopping(&PortHopConfig{
		StartPort:  30000,
		EndPort:    30010,
		TargetPort: 10080,
		Protocol:   "udp",
	})
	if err == nil {
		t.Fatal("expected remove failure")
	}
	if !errors.Is(err, backend.removeForwardErr) {
		t.Fatalf("expected joined error to include forward error, got %v", err)
	}
	if !errors.Is(err, backend.removeRangeErr) {
		t.Fatalf("expected joined error to include range error, got %v", err)
	}
}

func TestSetupPortHoppingRollsBackRangeWhenForwardFails(t *testing.T) {
	backend := &fakePortHopBackend{forwardErr: errors.New("forward failed")}
	m := &Manager{backend: backend}
	err := m.SetupPortHopping(&PortHopConfig{
		StartPort:  30000,
		EndPort:    30010,
		TargetPort: 10080,
		Protocol:   "udp",
	})
	if err == nil {
		t.Fatal("expected forwarding failure")
	}
	if !backend.addRangeCalled {
		t.Fatal("expected port range to be opened before forwarding")
	}
	if !backend.removeRangeCalled {
		t.Fatal("expected opened port range to be rolled back")
	}
}
