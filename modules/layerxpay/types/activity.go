package types

const (
	ActivityEscrowOpen           uint32 = 0x00020001
	ActivityEscrowCapture        uint32 = 0x00020002
	ActivityEscrowPartialCapture uint32 = 0x00020003
	ActivityEscrowRelease        uint32 = 0x00020004
	ActivityEscrowTimeout        uint32 = 0x00020005
	ActivityEscrowDisputeOpen    uint32 = 0x00020006
	ActivityEscrowDisputeResolve uint32 = 0x00020007

	ActivityBudgetCreate         uint32 = 0x00030001
	ActivityBudgetFund           uint32 = 0x00030002
	ActivityBudgetAmend          uint32 = 0x00030003
	ActivityBudgetDelegateAdd    uint32 = 0x00030004
	ActivityBudgetDelegateRemove uint32 = 0x00030005
	ActivityBudgetSpend          uint32 = 0x00030006
	ActivityBudgetClose          uint32 = 0x00030007
	ActivityBudgetDefund         uint32 = 0x00030008
	ActivityBudgetRevoke         uint32 = 0x00030009
	ActivityBudgetDelegateSpend  uint32 = 0x0003000a
	ActivityBudgetPull           uint32 = 0x0003000b

	ActivityStreamOpen       uint32 = 0x00040001
	ActivityStreamTopUp      uint32 = 0x00040002
	ActivityStreamMeter      uint32 = 0x00040003
	ActivityStreamSettle     uint32 = 0x00040004
	ActivityStreamPause      uint32 = 0x00040005
	ActivityStreamResume     uint32 = 0x00040006
	ActivityStreamClose      uint32 = 0x00040007
	ActivityStreamRateChange uint32 = 0x00040008

	ActivityServiceOfferPublish     uint32 = 0x00050001
	ActivityServiceOfferWithdraw    uint32 = 0x00050002
	ActivityServiceAgreementPropose uint32 = 0x00050003
	ActivityServiceAgreementAccept  uint32 = 0x00050004
	ActivityServiceCommitTask       uint32 = 0x00050005
	ActivityServiceCommitAbandon    uint32 = 0x00050006
	ActivityServiceToolExecAttest   uint32 = 0x00050007
	ActivityServiceProgressReport   uint32 = 0x00050008
	ActivityServiceDeliver          uint32 = 0x00050009
	ActivityServiceAccept           uint32 = 0x0005000a
	ActivityServiceReject           uint32 = 0x0005000b
	ActivityServiceDisputeOpen      uint32 = 0x0005000c
	ActivityServiceDisputeResolve   uint32 = 0x0005000d
)

type KernelActivity struct {
	Type         uint32
	Module       uint16
	PayloadBytes int
}

var MethodActivity = map[string]KernelActivity{
	"escrowOpen":           {ActivityEscrowOpen, EscrowModuleID, EscrowOpenPayloadBytes},
	"escrowCapture":        {ActivityEscrowCapture, EscrowModuleID, EscrowCapturePayloadBytes},
	"escrowPartialCapture": {ActivityEscrowPartialCapture, EscrowModuleID, EscrowCapturePayloadBytes},
	"escrowRelease":        {ActivityEscrowRelease, EscrowModuleID, EscrowReleasePayloadBytes},
	"escrowTimeout":        {ActivityEscrowTimeout, EscrowModuleID, EscrowReleasePayloadBytes},
	"escrowDisputeOpen":    {ActivityEscrowDisputeOpen, EscrowModuleID, EscrowDisputeOpenPayloadBytes},
	"escrowDisputeResolve": {ActivityEscrowDisputeResolve, EscrowModuleID, EscrowDisputeResolvePayloadBytes},

	"budgetCreate":         {ActivityBudgetCreate, BudgetModuleID, BudgetCreatePayloadBytes},
	"budgetCreateNative":   {ActivityBudgetCreate, BudgetModuleID, BudgetCreateNativePayloadBytes},
	"budgetFund":           {ActivityBudgetFund, BudgetModuleID, BudgetFundPayloadBytes},
	"budgetFundNative":     {ActivityBudgetFund, BudgetModuleID, BudgetFundNativePayloadBytes},
	"budgetAmend":          {ActivityBudgetAmend, BudgetModuleID, BudgetAmendPayloadBytes},
	"budgetDelegateAdd":    {ActivityBudgetDelegateAdd, BudgetModuleID, BudgetDelegatePayloadBytes},
	"budgetDelegateRemove": {ActivityBudgetDelegateRemove, BudgetModuleID, BudgetDelegatePayloadBytes},
	"budgetSpend":          {ActivityBudgetSpend, BudgetModuleID, BudgetSpendPayloadBytes},
	"budgetClose":          {ActivityBudgetClose, BudgetModuleID, BudgetClosePayloadBytes},
	"budgetDefund":         {ActivityBudgetDefund, BudgetModuleID, BudgetDefundPayloadBytes},
	"budgetRevoke":         {ActivityBudgetRevoke, BudgetModuleID, BudgetRevokePayloadBytes},
	"budgetDelegateSpend":  {ActivityBudgetDelegateSpend, BudgetModuleID, BudgetDelegateSpendPayloadBytes},
	"budgetPull":           {ActivityBudgetPull, BudgetModuleID, BudgetPullPayloadBytes},

	"streamOpen":   {ActivityStreamOpen, StreamModuleID, 0},
	"streamTopUp":  {ActivityStreamTopUp, StreamModuleID, StreamTopUpPayloadBytes},
	"streamMeter":  {ActivityStreamMeter, StreamModuleID, StreamMeterPayloadBytes},
	"streamSettle": {ActivityStreamSettle, StreamModuleID, StreamKeyedPayloadBytes},
	"streamPause":  {ActivityStreamPause, StreamModuleID, StreamIDPayloadBytes},
	"streamResume": {ActivityStreamResume, StreamModuleID, StreamIDPayloadBytes},
	"streamClose":  {ActivityStreamClose, StreamModuleID, StreamKeyedPayloadBytes},

	"serviceOfferPublish":     {ActivityServiceOfferPublish, ServiceModuleID, 179},
	"serviceOfferWithdraw":    {ActivityServiceOfferWithdraw, ServiceModuleID, 34},
	"serviceAgreementPropose": {ActivityServiceAgreementPropose, ServiceModuleID, 130},
	"serviceAgreementAccept":  {ActivityServiceAgreementAccept, ServiceModuleID, 34},
	"serviceCommitTask":       {ActivityServiceCommitTask, ServiceModuleID, 146},
	"serviceCommitAbandon":    {ActivityServiceCommitAbandon, ServiceModuleID, 36},
	"serviceToolExecAttest":   {ActivityServiceToolExecAttest, ServiceModuleID, ServiceExecutionBytes},
	"serviceProgressReport":   {ActivityServiceProgressReport, ServiceModuleID, 134},
	"serviceDeliver":          {ActivityServiceDeliver, ServiceModuleID, 0},
	"serviceAccept":           {ActivityServiceAccept, ServiceModuleID, 34},
	"serviceReject":           {ActivityServiceReject, ServiceModuleID, 0},
	"serviceDisputeOpen":      {ActivityServiceDisputeOpen, ServiceModuleID, 0},
	"serviceDisputeResolve":   {ActivityServiceDisputeResolve, ServiceModuleID, 72},
}
