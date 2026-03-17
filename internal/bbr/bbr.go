// Package bbr provides BBR congestion control and kernel management utilities.
// Supports 32 operations including kernel installation, acceleration enablement,
// system optimization, and kernel management.
package bbr

// KernelType represents a Linux kernel variant.
type KernelType string

const (
	KernelBBR       KernelType = "bbr"
	KernelBBRPlus   KernelType = "bbrplus"
	KernelLotServer KernelType = "lotserver"
	KernelZen       KernelType = "zen"
	KernelCloud     KernelType = "cloud"
	KernelXanmod    KernelType = "xanmod"
)

// CongestionControl represents a TCP congestion control algorithm.
type CongestionControl string

const (
	CCBbr     CongestionControl = "bbr"
	CCBbr2    CongestionControl = "bbr2"
	CCBbrPlus CongestionControl = "bbrplus"
	CCCubic   CongestionControl = "cubic"
	CCBrutal  CongestionControl = "brutal"
)

// QueueDiscipline represents a packet scheduling algorithm.
type QueueDiscipline string

const (
	QDiscFQ    QueueDiscipline = "fq"
	QDiscFQPie QueueDiscipline = "fq_pie"
	QDiscCake  QueueDiscipline = "cake"
)
