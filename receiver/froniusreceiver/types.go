package froniusreceiver

import "time"

// ======================== Generic Response Envelope ========================

// ResponseEnvelope ist der generische Wrapper für alle Fronius API Responses.
type ResponseEnvelope struct {
	Head Head        `json:"Head"`
	Body BodyWrapper `json:"Body"`
}

// Head enthält Metadaten der Response (Status, TimeStamp).
type Head struct {
	Status    Status `json:"Status"`
	TimeStamp string `json:"TimeStamp"`
}

// Status enthält den Responsestatus.
type Status struct {
	Code        int    `json:"Code"`
	Reason      string `json:"Reason,omitempty"`
	UserMessage string `json:"UserMessage,omitempty"`
}

// BodyWrapper ist generischer Wrapper für die Data.
type BodyWrapper struct {
	Data interface{} `json:"Data"`
}

// ======================== PowerFlow Realtime Data ========================

// PowerFlowRealtimeData ist die Response-Struktur für GetPowerFlowRealtimeData.fcgi.
type PowerFlowRealtimeData struct {
	Inverters map[string]PowerFlowInverter `json:"inverters"`
	Site      PowerFlowSite                `json:"Site"`
	Version   string                       `json:"Version,omitempty"`
}

// PowerFlowInverter enthält Inverter-Level Powerflow-Daten (aus Site-Sicht).
type PowerFlowInverter struct {
	DT           float64 `json:"DT"`
	P            float64 `json:"P"`       // AC Power in W
	E_Day        float64 `json:"E_Day"`   // Energy today in Wh
	E_Year       float64 `json:"E_Year"`  // Energy this year in Wh
	E_Total      float64 `json:"E_Total"` // Lifetime energy in Wh
	SOC          float64 `json:"SOC"`     // State of Charge % (battery)
	Battery_Mode string  `json:"Battery_Mode,omitempty"`
}

// PowerFlowSite enthält Site-Level Powerflow-Daten.
type PowerFlowSite struct {
	BackupMode          *bool   `json:"BackupMode"`
	BatteryStandby      *bool   `json:"BatteryStandby,omitempty"`
	E_Day               float64 `json:"E_Day"`   // Energy generated today in Wh
	E_Year              float64 `json:"E_Year"`  // Energy generated this year in Wh
	E_Total             float64 `json:"E_Total"` // Lifetime energy in Wh
	Meter_Location      string  `json:"Meter_Location"`
	Mode                string  `json:"Mode"`                // e.g., "Autonomy"
	P_Akku              float64 `json:"P_Akku"`              // Battery power in W
	P_Grid              float64 `json:"P_Grid"`              // Grid power in W (negative = import)
	P_Load              float64 `json:"P_Load"`              // Load power in W
	P_PV                float64 `json:"P_PV"`                // PV power in W
	Rel_Autonomy        float64 `json:"rel_Autonomy"`        // Autonomy ratio 0-100%
	Rel_SelfConsumption float64 `json:"rel_SelfConsumption"` // Self-consumption ratio 0-100%
}

// ======================== Inverter Realtime Data ========================

// InverterRealtimeData ist die Response-Struktur für GetInverterRealtimeData.cgi.
type InverterRealtimeData struct {
	// CommonInverterData fields (when DataCollection="CommonInverterData")
	DAY_ENERGY   *DataPoint    `json:"DAY_ENERGY,omitempty"`
	YEAR_ENERGY  *DataPoint    `json:"YEAR_ENERGY,omitempty"`
	TOTAL_ENERGY *DataPoint    `json:"TOTAL_ENERGY,omitempty"`
	PAC          *DataPoint    `json:"PAC,omitempty"`   // AC Power
	SAC          *DataPoint    `json:"SAC,omitempty"`   // Apparent Power
	IAC          *DataPoint    `json:"IAC,omitempty"`   // AC Current
	IDC          *DataPoint    `json:"IDC,omitempty"`   // DC Current String 1
	IDC_2        *DataPoint    `json:"IDC_2,omitempty"` // DC Current String 2
	IDC_3        *DataPoint    `json:"IDC_3,omitempty"` // DC Current String 3
	UAC          *DataPoint    `json:"UAC,omitempty"`   // AC Voltage
	UDC          *DataPoint    `json:"UDC,omitempty"`   // DC Voltage String 1
	UDC_2        *DataPoint    `json:"UDC_2,omitempty"` // DC Voltage String 2
	UDC_3        *DataPoint    `json:"UDC_3,omitempty"` // DC Voltage String 3
	FAC          *DataPoint    `json:"FAC,omitempty"`   // AC Frequency
	DeviceStatus *DeviceStatus `json:"DeviceStatus,omitempty"`

	// 3PInverterData fields (when DataCollection="3PInverterData")
	IAC_L1    *DataPoint `json:"IAC_L1,omitempty"`
	IAC_L2    *DataPoint `json:"IAC_L2,omitempty"`
	IAC_L3    *DataPoint `json:"IAC_L3,omitempty"`
	UAC_L1    *DataPoint `json:"UAC_L1,omitempty"`
	UAC_L2    *DataPoint `json:"UAC_L2,omitempty"`
	UAC_L3    *DataPoint `json:"UAC_L3,omitempty"`
	T_AMBIENT *DataPoint `json:"T_AMBIENT,omitempty"`
}

// DataPoint enthält einen Wert mit Unit.
type DataPoint struct {
	Value float64 `json:"Value"`
	Unit  string  `json:"Unit"`
}

// DeviceStatus enthält Statusinfos des Inverters.
type DeviceStatus struct {
	ErrorCode     int    `json:"ErrorCode"`
	InverterState string `json:"InverterState"`
	StatusCode    int    `json:"StatusCode"`
}

// ======================== Meter Realtime Data ========================

// MeterRealtimeData ist die Response-Struktur für GetMeterRealtimeData.cgi.
// Es ist eine Map, da es mehrere Meter geben kann.
type MeterRealtimeData map[string]MeterData

// MeterData enthält alle Meter-Felder (flexibel für verschiedene Meter-Typen).
type MeterData struct {
	Details                           *MeterDetails `json:"Details,omitempty"`
	Enable                            *float64      `json:"Enable,omitempty"`
	Visible                           *float64      `json:"Visible,omitempty"`
	Meter_Location_Current            *float64      `json:"Meter_Location_Current,omitempty"`
	Timestamp                         *float64      `json:"Timestamp,omitempty"`
	Current_AC_Phase_1                *float64      `json:"Current_AC_Phase_1,omitempty"`
	Current_AC_Phase_2                *float64      `json:"Current_AC_Phase_2,omitempty"`
	Current_AC_Phase_3                *float64      `json:"Current_AC_Phase_3,omitempty"`
	Current_AC_Sum                    *float64      `json:"Current_AC_Sum,omitempty"`
	Voltage_AC_Phase_1                *float64      `json:"Voltage_AC_Phase_1,omitempty"`
	Voltage_AC_Phase_2                *float64      `json:"Voltage_AC_Phase_2,omitempty"`
	Voltage_AC_Phase_3                *float64      `json:"Voltage_AC_Phase_3,omitempty"`
	Voltage_AC_PhaseToPhase_12        *float64      `json:"Voltage_AC_PhaseToPhase_12,omitempty"`
	Voltage_AC_PhaseToPhase_23        *float64      `json:"Voltage_AC_PhaseToPhase_23,omitempty"`
	Voltage_AC_PhaseToPhase_31        *float64      `json:"Voltage_AC_PhaseToPhase_31,omitempty"`
	Voltage_AC_Phase_Average          *float64      `json:"Voltage_AC_Phase_Average,omitempty"`
	PowerReal_P_Phase_1               *float64      `json:"PowerReal_P_Phase_1,omitempty"`
	PowerReal_P_Phase_2               *float64      `json:"PowerReal_P_Phase_2,omitempty"`
	PowerReal_P_Phase_3               *float64      `json:"PowerReal_P_Phase_3,omitempty"`
	PowerReal_P_Sum                   *float64      `json:"PowerReal_P_Sum,omitempty"`
	PowerReactive_Q_Phase_1           *float64      `json:"PowerReactive_Q_Phase_1,omitempty"`
	PowerReactive_Q_Phase_2           *float64      `json:"PowerReactive_Q_Phase_2,omitempty"`
	PowerReactive_Q_Phase_3           *float64      `json:"PowerReactive_Q_Phase_3,omitempty"`
	PowerReactive_Q_Sum               *float64      `json:"PowerReactive_Q_Sum,omitempty"`
	PowerApparent_S_Phase_1           *float64      `json:"PowerApparent_S_Phase_1,omitempty"`
	PowerApparent_S_Phase_2           *float64      `json:"PowerApparent_S_Phase_2,omitempty"`
	PowerApparent_S_Phase_3           *float64      `json:"PowerApparent_S_Phase_3,omitempty"`
	PowerApparent_S_Sum               *float64      `json:"PowerApparent_S_Sum,omitempty"`
	PowerFactor_Phase_1               *float64      `json:"PowerFactor_Phase_1,omitempty"`
	PowerFactor_Phase_2               *float64      `json:"PowerFactor_Phase_2,omitempty"`
	PowerFactor_Phase_3               *float64      `json:"PowerFactor_Phase_3,omitempty"`
	PowerFactor_Sum                   *float64      `json:"PowerFactor_Sum,omitempty"`
	Frequency_Phase_Average           *float64      `json:"Frequency_Phase_Average,omitempty"`
	EnergyReal_WAC_Sum_Consumed       *float64      `json:"EnergyReal_WAC_Sum_Consumed,omitempty"`
	EnergyReal_WAC_Sum_Produced       *float64      `json:"EnergyReal_WAC_Sum_Produced,omitempty"`
	EnergyReal_WAC_Plus_Absolute      *float64      `json:"EnergyReal_WAC_Plus_Absolute,omitempty"`
	EnergyReal_WAC_Minus_Absolute     *float64      `json:"EnergyReal_WAC_Minus_Absolute,omitempty"`
	EnergyReal_WAC_Phase_1_Consumed   *float64      `json:"EnergyReal_WAC_Phase_1_Consumed,omitempty"`
	EnergyReal_WAC_Phase_1_Produced   *float64      `json:"EnergyReal_WAC_Phase_1_Produced,omitempty"`
	EnergyReal_WAC_Phase_2_Consumed   *float64      `json:"EnergyReal_WAC_Phase_2_Consumed,omitempty"`
	EnergyReal_WAC_Phase_2_Produced   *float64      `json:"EnergyReal_WAC_Phase_2_Produced,omitempty"`
	EnergyReal_WAC_Phase_3_Consumed   *float64      `json:"EnergyReal_WAC_Phase_3_Consumed,omitempty"`
	EnergyReal_WAC_Phase_3_Produced   *float64      `json:"EnergyReal_WAC_Phase_3_Produced,omitempty"`
	EnergyReactive_VArAC_Sum_Consumed *float64      `json:"EnergyReactive_VArAC_Sum_Consumed,omitempty"`
	EnergyReactive_VArAC_Sum_Produced *float64      `json:"EnergyReactive_VArAC_Sum_Produced,omitempty"`
}

// MeterDetails enthält Metadaten des Meters.
type MeterDetails struct {
	Manufacturer string `json:"Manufacturer"`
	Model        string `json:"Model"`
	Serial       string `json:"Serial"`
}

// ======================== Storage/Battery Realtime Data ========================

// StorageRealtimeData ist die Response-Struktur für GetStorageRealtimeData.cgi.
type StorageRealtimeData map[string]StorageDevice

// StorageDevice enthält Storage/Battery-Daten.
type StorageDevice struct {
	Controller StorageController `json:"Controller"`
	Modules    []interface{}     `json:"Modules"` // Optional, kann leer sein
}

// StorageController enthält den Battery-Controller Status.
type StorageController struct {
	Details                StorageDetails `json:"Details"`
	Capacity_Maximum       float64        `json:"Capacity_Maximum"`       // Max Kapazität in Wh
	Current_DC             float64        `json:"Current_DC"`             // Strom in A
	DesignedCapacity       float64        `json:"DesignedCapacity"`       // Designed Kapazität in Wh
	Enable                 float64        `json:"Enable"`                 // 0 oder 1
	StateOfCharge_Relative float64        `json:"StateOfCharge_Relative"` // SOC in %
	Status_BatteryCell     float64        `json:"Status_BatteryCell"`     // Status Code
	Temperature_Cell       float64        `json:"Temperature_Cell"`       // Temperatur in °C
	TimeStamp              float64        `json:"TimeStamp"`              // Unix Timestamp
	Voltage_DC             float64        `json:"Voltage_DC"`             // Spannung in V
}

// StorageDetails enthält Metadaten des Storage-Controllers.
type StorageDetails struct {
	Manufacturer string `json:"Manufacturer"`
	Model        string `json:"Model"`
	Serial       string `json:"Serial"`
}

// ======================== Inverter Info ========================

// InverterInfoData ist die Response-Struktur für GetInverterInfo.cgi.
type InverterInfoData map[string]InverterInfo

// InverterInfo enthält Metadaten über einen Inverter.
type InverterInfo struct {
	CustomName    string  `json:"CustomName"`
	DT            float64 `json:"DT"`
	ErrorCode     int     `json:"ErrorCode"`
	InverterState string  `json:"InverterState"`
	PVPower       float64 `json:"PVPower"` // Rated PV Power
	Show          float64 `json:"Show"`    // 0 oder 1
	StatusCode    int     `json:"StatusCode"`
	UniqueID      string  `json:"UniqueID"`
}

// ======================== Active Device Info ========================

// ActiveDeviceInfo ist die Response-Struktur für GetActiveDeviceInfo.cgi.
type ActiveDeviceInfo struct {
	Inverter map[string]DeviceEntry `json:"Inverter,omitempty"`
	Meter    map[string]DeviceEntry `json:"Meter,omitempty"`
	Storage  map[string]DeviceEntry `json:"Storage,omitempty"`
	Ohmpilot map[string]DeviceEntry `json:"Ohmpilot,omitempty"`
}

// DeviceEntry enthält Geräteinformation (Serial, DT).
type DeviceEntry struct {
	DT     float64 `json:"DT"`
	Serial string  `json:"Serial"`
}

// ======================== Ohmpilot Realtime Data ========================

// OhmpilotRealtimeData ist die Response-Struktur für GetOhmpilotRealtimeData.cgi.
// Map per DeviceId.
type OhmpilotRealtimeData map[string]OhmpilotDevice

// OhmpilotDevice enthält die Daten eines einzelnen Ohmpilots.
type OhmpilotDevice struct {
	Details                               OhmpilotDetails `json:"Details"`
	CodeOfState                           float64         `json:"CodeOfState"`
	EnergyReal_WAC_Sum_Consumed           float64         `json:"EnergyReal_WAC_Sum_Consumed"`           // Wh
	PowerReal_PAC_Sum                     float64         `json:"PowerReal_PAC_Sum"`                     // W
	Temperature_Channel_1                 float64         `json:"Temperature_Channel_1"`                 // °C
	EnergyReactive_VArAC_Phase_1_Produced float64         `json:"EnergyReactive_VArAC_Phase_1_Produced"` // varh
}

// OhmpilotDetails enthält Metadaten des Ohmpilot.
type OhmpilotDetails struct {
	Manufacturer string `json:"Manufacturer"`
	Model        string `json:"Model"`
	Serial       string `json:"Serial"`
	Hardware     string `json:"Hardware,omitempty"`
	Software     string `json:"Software,omitempty"`
}

// ======================== Inverter Realtime per Device ========================

// InverterRealtimeMap mappt Inverter-IDs auf ihre Realtime-Daten.
// Wird gebraucht, da pro Inverter ein eigener API-Call mit DeviceId nötig ist.
type InverterRealtimeMap map[string]*InverterRealtimeData

// ======================== ScrapeStats ========================

// ScrapeStats enthält Telemetry-Daten über den Scrape-Zyklus selbst.
type ScrapeStats struct {
	DurationSeconds float64 // Gesamt-Dauer des Scrape-Zyklus
	Errors          int64   // Anzahl Fehler in diesem Zyklus
	Success         bool    // true falls mindestens ein Endpoint erfolgreich
}

// ======================== ScrapedMetrics ========================

// ScrapedMetrics fasst alle gescrapten Daten von einem Scrape-Zyklus zusammen.
type ScrapedMetrics struct {
	Timestamp time.Time
	PowerFlow *PowerFlowRealtimeData
	Inverters InverterRealtimeMap // pro Device-ID
	Meter     MeterRealtimeData
	Storage   StorageRealtimeData
	Ohmpilot  OhmpilotRealtimeData
	Info      InverterInfoData
	Stats     ScrapeStats
}
