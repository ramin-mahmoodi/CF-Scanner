package gui

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XIU2/CloudflareSpeedTest/task"
	"github.com/XIU2/CloudflareSpeedTest/utils"
)

type XrayResult struct {
	IP     string
	Config string
	Score  float64
	Delay  int
}

var xrayTableData []XrayResult
var xrayVBox *fyne.Container
var xrayScroll *container.Scroll

var (
	currentTabIndex int
	fyneApp         fyne.App
	fyneWin         fyne.Window

	progressBinding binding.Float
	statusBinding   binding.String
	resultsData     []utils.CloudflareIPData
	isRunning       bool
	isDarkTheme     bool = true

	btnStart *widget.Button

	inputThreads               *widget.Entry
	inputPingCount             *widget.Entry
	inputPort                  *widget.Entry
	lblSNI                     *widget.Label
	inputSNI                   *widget.Entry
	inputCustomIPs             *widget.Entry
	scrollIPs                  *container.Scroll
	ipBackground               *widget.Entry
	btnSelectAll               *widget.Button
	btnClearAll                *widget.Button
	btnFetchIPv4               *widget.Button
	ipContainer                *fyne.Container
	selectScanMode             *widget.Select
	checkTLS                   *widget.Check
	checkHTTP                  *widget.Check
	inputIPCount               *widget.Entry
	inputXrayThreads           *widget.Entry
	btnThemeToggle             *widget.Button
	selectLang                 *widget.Select

	lblThreads, lblPingCount, lblPort, lblMode, lblIPs, lblCustomIPs, lblIPCount, lblScanMode, lblXrayConfig, lblXrayCore, lblCheckXray, lblXrayThreads *widget.Label
	inputXrayConfig                                                                                                                                     *widget.Entry
	btnCheckXray                                                                                                                                        *widget.Button
	testHeaderBtns                                                                                                                                      [5]*widget.Button
	btnCopyAlive, btnExportAlive                                                                                                                        *widget.Button

	defaultIPRanges = []string{
		"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
		"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
		"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/12",
		"172.64.0.0/17", "172.64.128.0/18", "172.64.192.0/19", "172.64.224.0/22",
		"172.64.229.0/24", "172.64.230.0/23", "172.64.232.0/21", "172.64.240.0/21",
		"172.64.248.0/21", "172.65.0.0/16", "172.66.0.0/16", "172.67.0.0/16", "131.0.72.0/22",
	}
	selectedIPRanges = append([]string{}, defaultIPRanges...)

	headerBtns [8]*widget.Button
	dataVBox   *fyne.Container
	dataScroll *container.Scroll

	lblExportBaseURI, lblExportCount     *widget.Label
	inputExportBaseURI, inputExportCount *widget.Entry
	btnExportGenerate                    *widget.Button
	outputExportConfigs                  *widget.Entry

	settingsContainer *fyne.Container
	settingsBorder    *canvas.Rectangle
	tableBorder       *canvas.Rectangle
	bottomBorder      *canvas.Rectangle
	contentContainer  *fyne.Container
	settingsContent   fyne.CanvasObject
	resultsContent    fyne.CanvasObject

	btnTabSettings *widget.Button
	btnTabResults  *widget.Button
	btnTabExport   *widget.Button
	btnTabTest     *widget.Button
	exportContent  fyne.CanvasObject
	testContent    fyne.CanvasObject

	inputTestConfigs *widget.Entry
	btnTestStart     *widget.Button

	statusIcon *widget.Icon

	lastSortCol int  = -1
	sortAsc     bool = false

	refreshMutex sync.Mutex
	refreshTimer *time.Timer
)

func throttleRefresh() {
	refreshMutex.Lock()
	defer refreshMutex.Unlock()
	if refreshTimer == nil {
		refreshTimer = time.AfterFunc(150*time.Millisecond, func() {
			refreshMutex.Lock()
			refreshTimer = nil
			refreshMutex.Unlock()
			if dataVBox != nil {
				refreshDataVBox()
			}
		})
	}
}

func refreshDataVBox() {
	if dataVBox == nil {
		return
	}

	headers := []string{T("col_ip"), T("col_sent"), T("col_recv"), T("col_loss"), T("col_delay"), T("col_jitter"), T("col_score"), T("col_colo")}

	// Recycle existing objects if length matches (avoids lag during scan and sorting)
	if len(dataVBox.Objects) > 0 && len(dataVBox.Objects) == len(resultsData)+1 {
		for id, data := range resultsData {
			rowContainer := dataVBox.Objects[id+1].(*fyne.Container)
			rowContent := rowContainer.Objects[1].(*fyne.Container)

			for i := 0; i < 8; i++ {
				cellStack := rowContent.Objects[i].(*fyne.Container)
				cellBg := cellStack.Objects[0].(*canvas.Rectangle)
				lbl := cellStack.Objects[1].(*widget.Label)
				cellBg.FillColor = color.Transparent

				switch i {
				case 0:
					lbl.SetText(data.IP.String() + "\u200E")
				case 1:
					lbl.SetText(strconv.Itoa(data.Sended) + "\u200E")
				case 2:
					lbl.SetText(strconv.Itoa(data.Received) + "\u200E")
				case 3:
					if data.Sended > 0 {
						lossRate := float64(data.Sended-data.Received) / float64(data.Sended)
						lbl.SetText(strconv.FormatFloat(lossRate*100, 'f', 2, 64) + "%\u200E")
						if lossRate > 0 {
							cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
						}
					} else {
						lbl.SetText("100%\u200E")
					}
				case 4:
					delayMs := data.Delay.Seconds() * 1000
					lbl.SetText(strconv.FormatFloat(delayMs, 'f', 2, 64) + " ms\u200E")
					if delayMs < 150 && delayMs > 0 {
						cellBg.FillColor = color.NRGBA{R: 100, G: 255, B: 100, A: 50}
					} else if delayMs >= 150 && delayMs <= 250 {
						cellBg.FillColor = color.NRGBA{R: 255, G: 255, B: 100, A: 50}
					} else if delayMs > 250 {
						cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
					}
				case 5:
					if data.Received <= 1 {
						lbl.SetText("N/A")
					} else {
						jitterMs := float64(data.Jitter.Microseconds()) / 1000.0
						lbl.SetText(strconv.FormatFloat(jitterMs, 'f', 2, 64) + " ms\u200E")
					}
				case 6:
					lbl.SetText(strconv.FormatFloat(data.Score, 'f', 2, 64) + "\u200E")
				case 7:
					if data.Colo == "" {
						lbl.SetText("N/A")
					} else {
						lbl.SetText(data.Colo)
					}
				}
			}
		}
		dataVBox.Refresh()
		return
	}

	dataVBox.Objects = nil
	colWidths := []float32{160, 130, 130, 130, 130, 130, 130, 130}
	headerCells := make([]fyne.CanvasObject, 8)
	for i := 0; i < 8; i++ {
		colIndex := i
		btn := widget.NewButton(headers[i], func() {
			sortResultsData(colIndex)
			refreshDataVBox()
		})
		btn.Importance = widget.LowImportance
		headerBtns[i] = btn
		headerCells[i] = btn
	}
	headerContainer := container.New(&tableRowLayout{widths: colWidths}, headerCells...)
	dataVBox.Add(headerContainer)

	for id, data := range resultsData {
		rowBg := canvas.NewRectangle(color.Transparent)
		if id%2 == 1 {
			rowBg.FillColor = color.NRGBA{R: 128, G: 128, B: 128, A: 20}
		}

		cells := make([]fyne.CanvasObject, 8)
		for i := 0; i < 8; i++ {
			cellBg := canvas.NewRectangle(color.Transparent)
			lbl := widget.NewLabel("")
			lbl.Alignment = fyne.TextAlignCenter

			switch i {
			case 0:
				lbl.SetText(data.IP.String() + "\u200E")
			case 1:
				lbl.SetText(strconv.Itoa(data.Sended) + "\u200E")
			case 2:
				lbl.SetText(strconv.Itoa(data.Received) + "\u200E")
			case 3:
				if data.Sended > 0 {
					lossRate := float64(data.Sended-data.Received) / float64(data.Sended)
					lbl.SetText(strconv.FormatFloat(lossRate*100, 'f', 2, 64) + "%\u200E")
					if lossRate > 0 {
						cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
					}
				} else {
					lbl.SetText("100%\u200E")
				}
			case 4:
				delayMs := data.Delay.Seconds() * 1000
				lbl.SetText(strconv.FormatFloat(delayMs, 'f', 2, 64) + " ms\u200E")
				if delayMs < 150 && delayMs > 0 {
					cellBg.FillColor = color.NRGBA{R: 100, G: 255, B: 100, A: 50}
				} else if delayMs >= 150 && delayMs <= 250 {
					cellBg.FillColor = color.NRGBA{R: 255, G: 255, B: 100, A: 50}
				} else if delayMs > 250 {
					cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
				}
			case 5:
				if data.Received <= 1 {
					lbl.SetText("N/A")
				} else {
					jitterMs := float64(data.Jitter.Microseconds()) / 1000.0
					lbl.SetText(strconv.FormatFloat(jitterMs, 'f', 2, 64) + " ms\u200E")
				}
			case 6:
				lbl.SetText(strconv.FormatFloat(data.Score, 'f', 2, 64) + "\u200E")
			case 7:
				if data.Colo == "" {
					lbl.SetText("N/A")
				} else {
					lbl.SetText(data.Colo)
				}
			}
			cells[i] = container.NewStack(cellBg, lbl)
		}
		rowContent := container.New(&tableRowLayout{widths: colWidths}, cells...)
		dataVBox.Add(container.NewStack(rowBg, rowContent))
	}
	dataVBox.Refresh()
}

type tableRowLayout struct {
	widths []float32
}

func (l *tableRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width, height float32
	for i, w := range l.widths {
		width += w
		if len(objects) > i {
			h := objects[i].MinSize().Height
			if h > height {
				height = h
			}
		}
	}
	return fyne.NewSize(width, height)
}

func (l *tableRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for i, w := range l.widths {
		if len(objects) > i {
			objects[i].Resize(fyne.NewSize(w, size.Height))
			objects[i].Move(fyne.NewPos(x, 0))
		}
		x += w
	}
}

type thinBarLayout struct{}

func (t *thinBarLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(100, 18)
}

func (t *thinBarLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, child := range objects {
		child.Resize(fyne.NewSize(size.Width, 18))
		child.Move(fyne.NewPos(0, (size.Height-18)/2))
	}
}

// clickBlocker is a dummy transparent widget to absorb clicks
type clickBlocker struct {
	widget.BaseWidget
}

func newClickBlocker() *clickBlocker {
	b := &clickBlocker{}
	b.ExtendBaseWidget(b)
	return b
}

func (b *clickBlocker) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (b *clickBlocker) Tapped(*fyne.PointEvent) {} // Consume click

func Start() {
	fyneApp = app.New()
	fyneApp.SetIcon(resourceLogoPng)
	fyneWin = fyneApp.NewWindow("CF Scanner")
	fyneWin.Resize(fyne.NewSize(850, 600))

	fontRes := fyne.NewStaticResource("Vazirmatn.ttf", vazirmatnFontBytes)
	customTheme := NewCustomTheme(isDarkTheme, fontRes)
	fyneApp.Settings().SetTheme(customTheme)

	progressBinding = binding.NewFloat()
	statusBinding = binding.NewString()
	statusBinding.Set(T("status_ready"))

	utils.GuiProgressCallback = func(current, total int) {
		if total > 0 {
			progressBinding.Set(float64(current) / float64(total))
		}
	}

	utils.GuiLiveCallback = func(data utils.CloudflareIPData) {
		resultsData = append(resultsData, data)
		throttleRefresh()
	}

	utils.GuiSpeedCallback = func() {
		throttleRefresh()
	}

	buildUI()

	fyneWin.CenterOnScreen()
	fyneWin.ShowAndRun()
}

func withPadding(obj fyne.CanvasObject, pad float32) fyne.CanvasObject {
	top := canvas.NewRectangle(color.Transparent)
	top.SetMinSize(fyne.NewSize(1, pad))
	bottom := canvas.NewRectangle(color.Transparent)
	bottom.SetMinSize(fyne.NewSize(1, pad))
	left := canvas.NewRectangle(color.Transparent)
	left.SetMinSize(fyne.NewSize(pad, 1))
	right := canvas.NewRectangle(color.Transparent)
	right.SetMinSize(fyne.NewSize(pad, 1))
	return container.NewBorder(top, bottom, left, right, obj)
}

func buildUI() {
	inputThreads = widget.NewEntry()
	inputThreads.SetText("200")
	inputPingCount = widget.NewEntry()
	inputPingCount.SetText("4")
	inputIPCount = widget.NewEntry()
	inputIPCount.SetText("20")
	inputPort = widget.NewEntry()
	inputPort.SetText("443")

	lblSNI = widget.NewLabel(T("sni"))
	inputSNI = widget.NewEntry()
	inputSNI.SetText("speed.cloudflare.com")

	// Fixed height scrolling box for IP selection (always visible)
	checkGroup := widget.NewCheckGroup(defaultIPRanges, func(s []string) {
		selectedIPRanges = s
	})
	checkGroup.SetSelected(selectedIPRanges)
	scrollIPs = container.NewVScroll(checkGroup)
	scrollIPs.SetMinSize(fyne.NewSize(200, 120)) // Fixed compact height

	ipBackground = widget.NewMultiLineEntry()
	// A click blocker to prevent the background entry from gaining focus or receiving typing
	blocker := newClickBlocker()
	// Add some padding so the IP text doesn't touch the exact edge of the text box
	paddedScroll := container.NewPadded(scrollIPs)

	scrollWithBg := container.NewStack(ipBackground, blocker, paddedScroll)

	btnSelectAll = widget.NewButton(T("select_all"), func() {
		checkGroup.SetSelected(defaultIPRanges)
	})
	btnClearAll = widget.NewButton(T("clear_all"), func() {
		checkGroup.SetSelected([]string{})
	})
	inputCustomIPs = widget.NewMultiLineEntry()
	inputCustomIPs.SetPlaceHolder("8.6.112.0/24\n37.49.225.189...")

	btnFetchIPv4 = widget.NewButton(T("fetch_ipv4"), func() {
		go fetchIPv4(checkGroup)
	})

	ipControls := container.NewHBox(btnSelectAll, btnClearAll, btnFetchIPv4)
	ipContainer = container.NewVBox(scrollWithBg, ipControls)

	checkTLS = widget.NewCheck(T("test_tls"), nil)
	checkTLS.OnChanged = func(b bool) {
		task.TestTLS = b
		if b {
			inputSNI.Enable()
		} else {
			inputSNI.Disable()
		}
	}
	checkTLS.SetChecked(true)
	checkHTTP = widget.NewCheck(T("test_http"), nil)
	checkHTTP.OnChanged = func(b bool) {
		task.TestHTTP = b
	}
	checkHTTP.SetChecked(true)

	selectScanMode = widget.NewSelect([]string{T("scan_random"), T("scan_sequential")}, nil)
	selectScanMode.SetSelected(T("scan_random"))

	btnThemeToggle = widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
		isDarkTheme = !isDarkTheme
		fontRes := fyne.NewStaticResource("Vazirmatn.ttf", vazirmatnFontBytes)
		fyneApp.Settings().SetTheme(NewCustomTheme(isDarkTheme, fontRes))
		updateLabels()
	})

	selectLang = widget.NewSelect([]string{T("lang_en"), T("lang_fa")}, func(val string) {
		if val == T("lang_en") {
			SetLang(LangEN)
		} else {
			SetLang(LangFA)
		}
		updateLabels()
	})
	selectLang.SetSelected(T("lang_en"))

	lblThreads = widget.NewLabel(T("threads"))
	lblPingCount = widget.NewLabel(T("ping_count"))
	lblPort = widget.NewLabel(T("port"))
	checkTLS.SetText(T("test_tls"))
	checkHTTP.SetText(T("test_http"))
	lblIPCount = widget.NewLabel(T("ip_count"))
	lblScanMode = widget.NewLabel(T("scan_mode"))
	lblIPs = widget.NewLabel(T("select_ips"))
	lblCustomIPs = widget.NewLabel(T("custom_ips"))

	lblXrayThreads = widget.NewLabel(T("xray_threads"))
	inputXrayThreads = widget.NewEntry()
	inputXrayThreads.SetText("5")

	lblXrayConfig = widget.NewLabel(T("xray_config"))
	inputXrayConfig = widget.NewEntry()

	lblXrayCore = widget.NewLabel(T("xray_core"))
	lblCheckXray = widget.NewLabel("")
	btnCheckXray = widget.NewButton(T("xray_check"), func() {
		btnCheckXray.Disable()
		lblCheckXray.SetText("")

		if task.CheckXrayCore() {
			lblCheckXray.SetText(T("xray_found"))
			btnCheckXray.Enable()
			return
		}

		go func() {
			err := task.DownloadXrayCore(func(state string) {
				if state == "downloading" {
					lblCheckXray.SetText(T("xray_downloading"))
				} else if state == "extracting" {
					lblCheckXray.SetText(T("xray_extracting"))
				}
			})

			if err != nil {
				lblCheckXray.SetText(T("xray_fail") + " " + err.Error())
			} else {
				lblCheckXray.SetText(T("xray_success"))
			}
			btnCheckXray.Enable()
		}()
	})
	// checkXrayBox moved to updateLabels

	settingsContainer = container.NewVBox(
	// Initialized empty, will be populated by updateLabels()
	)

	// Use simple padded container (stretches natively)
	// Apply 12px internal padding so content isn't stuck to the border
	paddedSettings := withPadding(settingsContainer, 12)

	// Put VScroll OUTSIDE the padding so the scrollbar and shadows touch the border cleanly
	scrollableSettings := container.NewVScroll(paddedSettings)

	// Custom background with visible stroke border instead of widget.Card
	settingsBorder = canvas.NewRectangle(color.Transparent)
	settingsBorder.StrokeWidth = 1
	settingsBorder.CornerRadius = 8 // Make corners rounded

	// Draw settingsBorder ON TOP of the scrollable content so it doesn't get covered
	settingsCard := container.NewStack(scrollableSettings, settingsBorder)
	settingsContent = container.NewPadded(settingsCard)

	dataVBox = container.NewVBox()
	refreshDataVBox()

	tableBorder = canvas.NewRectangle(color.Transparent)
	tableBorder.StrokeWidth = 1
	tableBorder.CornerRadius = 8

	dataScroll = container.NewScroll(dataVBox)
	paddedTable := withPadding(dataScroll, 12)
	tableCard := container.NewStack(paddedTable, tableBorder)
	resultsContent = container.NewPadded(tableCard)
	contentContainer = container.NewStack(settingsContent)

	btnTabSettings = widget.NewButtonWithIcon(T("settings"), theme.SettingsIcon(), func() {
		switchTab(0)
	})
	btnTabSettings.Importance = widget.HighImportance

	btnTabResults = widget.NewButtonWithIcon(T("results_tab"), theme.ListIcon(), func() {
		switchTab(1)
	})

	btnTabTest = widget.NewButtonWithIcon(T("test_tab"), theme.SearchIcon(), func() {
		switchTab(2)
	})

	buildExportUI()
	buildTestUI()

	statusIcon = widget.NewIcon(nil)
	topBar := container.NewHBox(
		btnTabSettings,
		btnTabResults,
		btnTabTest,
		layout.NewSpacer(),
		selectLang,
		btnThemeToggle,
	)
	topBarContainer := container.NewPadded(container.NewHScroll(topBar))

	btnStart = widget.NewButtonWithIcon(T("start_test"), theme.MediaPlayIcon(), func() {
		if isRunning {
			if utils.CancelFunc != nil {
				utils.CancelFunc()
			}
			resetStartButton()
		} else {
			if currentTabIndex == 2 {
				go startXrayTest()
			} else {
				startTest()
			}
		}
	})
	btnStart.Importance = widget.SuccessImportance

	progBar := widget.NewProgressBarWithData(progressBinding)
	thinProgBarContainer := container.New(&thinBarLayout{}, progBar)

	midSpacer := canvas.NewRectangle(color.Transparent)
	midSpacer.SetMinSize(fyne.NewSize(1, 8))

	bottomControls := container.NewVBox(
		thinProgBarContainer,
		midSpacer,
		btnStart,
	)
	paddedBottomControls := withPadding(bottomControls, 12)

	bottomBorder = canvas.NewRectangle(color.Transparent)
	bottomBorder.StrokeWidth = 1
	bottomBorder.CornerRadius = 8
	if isDarkTheme {
		bottomBorder.StrokeColor = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
	} else {
		bottomBorder.StrokeColor = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	}

	bottomCard := container.NewStack(paddedBottomControls, bottomBorder)
	bottom := container.NewPadded(bottomCard)

	mainLayout := container.NewBorder(topBarContainer, bottom, nil, nil, contentContainer)

	minSizeSpacer := canvas.NewRectangle(color.Transparent)
	minSizeSpacer.SetMinSize(fyne.NewSize(500, 400))
	content := container.NewStack(minSizeSpacer, mainLayout)

	fyneWin.SetContent(content)

	updateLabels()
}

var (
	sortXrayAsc     bool
	lastSortXrayCol int = -1
)

func sortXrayData(col int) {
	if lastSortXrayCol == col {
		sortXrayAsc = !sortXrayAsc
	} else {
		sortXrayAsc = true
		lastSortXrayCol = col
	}

	sort.Slice(xrayTableData, func(i, j int) bool {
		var isLess bool
		switch col {
		case 0:
			isLess = xrayTableData[i].IP < xrayTableData[j].IP
		case 2:
			isLess = xrayTableData[i].Score < xrayTableData[j].Score
		case 3:
			isLess = xrayTableData[i].Delay < xrayTableData[j].Delay
		default:
			isLess = xrayTableData[i].IP < xrayTableData[j].IP
		}
		if !sortXrayAsc {
			return !isLess
		}
		return isLess
	})
}

func sortResultsData(col int) {
	if lastSortCol == col {
		sortAsc = !sortAsc
	} else {
		sortAsc = true
		lastSortCol = col
	}

	sort.Slice(resultsData, func(i, j int) bool {
		var isLess bool
		switch col {
		case 0:
			isLess = resultsData[i].IP.String() < resultsData[j].IP.String()
		case 1:
			isLess = resultsData[i].Sended < resultsData[j].Sended
		case 2:
			isLess = resultsData[i].Received < resultsData[j].Received
		case 3:
			lossI := 1.0
			if resultsData[i].Sended > 0 {
				lossI = float64(resultsData[i].Sended-resultsData[i].Received) / float64(resultsData[i].Sended)
			}
			lossJ := 1.0
			if resultsData[j].Sended > 0 {
				lossJ = float64(resultsData[j].Sended-resultsData[j].Received) / float64(resultsData[j].Sended)
			}
			isLess = lossI < lossJ
		case 4:
			isLess = resultsData[i].Delay < resultsData[j].Delay
		case 5:
			isLess = resultsData[i].Jitter < resultsData[j].Jitter
		case 6:
			isLess = resultsData[i].Score < resultsData[j].Score
		case 7:
			isLess = resultsData[i].Colo < resultsData[j].Colo
		}
		if sortAsc {
			return isLess
		}
		return !isLess
	})
}

func switchTab(index int) {
	contentContainer.Objects = nil
	if btnTabSettings != nil {
		btnTabSettings.Importance = widget.MediumImportance
	}
	if btnTabResults != nil {
		btnTabResults.Importance = widget.MediumImportance
	}
	if btnTabTest != nil {
		btnTabTest.Importance = widget.MediumImportance
	}

	if index == 0 {
		if btnTabSettings != nil {
			btnTabSettings.Importance = widget.HighImportance
		}
		if settingsContent != nil {
			contentContainer.Add(settingsContent)
		}
	} else if index == 1 {
		if btnTabResults != nil {
			btnTabResults.Importance = widget.HighImportance
		}
		if resultsContent != nil {
			contentContainer.Add(resultsContent)
		}
	} else if index == 2 {
		if btnTabTest != nil {
			btnTabTest.Importance = widget.HighImportance
		}
		if testContent != nil {
			contentContainer.Add(testContent)
		}
	}
	if btnTabSettings != nil {
		btnTabSettings.Refresh()
	}
	if btnTabResults != nil {
		btnTabResults.Refresh()
	}
	if btnTabTest != nil {
		btnTabTest.Refresh()
	}
	currentTabIndex = index
	if !isRunning && btnStart != nil {
		if index == 2 {
			btnStart.Importance = widget.WarningImportance
		} else {
			btnStart.Importance = widget.SuccessImportance
		}
		btnStart.Refresh()
	}

	if contentContainer != nil {
		contentContainer.Refresh()
	}
}

func resetStartButton() {
	isRunning = false
	if btnStart != nil {
		btnStart.SetText(T("start_test"))
		btnStart.SetIcon(theme.MediaPlayIcon())
		if currentTabIndex == 2 {
			btnStart.Importance = widget.WarningImportance
		} else {
			btnStart.Importance = widget.SuccessImportance
		}
		btnStart.Refresh()
	}
}

func setStopButton() {
	isRunning = true
	if btnStart != nil {
		btnStart.SetText(T("stop_test"))
		btnStart.SetIcon(theme.MediaStopIcon())
		btnStart.Importance = widget.DangerImportance
		btnStart.Refresh()
	}
}

func makeRow(lbl *widget.Label, input fyne.CanvasObject) *fyne.Container {
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(200, 0))

	if currentLang == LangFA {
		lbl.Alignment = fyne.TextAlignTrailing
		rightSide := container.NewStack(spacer, lbl)
		return container.NewPadded(container.NewBorder(nil, nil, nil, rightSide, input))
	} else {
		lbl.Alignment = fyne.TextAlignLeading
		leftSide := container.NewStack(spacer, lbl)
		return container.NewPadded(container.NewBorder(nil, nil, leftSide, nil, input))
	}
}

func updateLabels() {
	if settingsContainer != nil {
		checkXrayBox := container.NewHBox(btnCheckXray, lblCheckXray)
		settingsContainer.Objects = []fyne.CanvasObject{
			makeRow(lblThreads, inputThreads),
			makeRow(lblPingCount, inputPingCount),
			makeRow(lblPort, inputPort),
			makeRow(lblSNI, inputSNI),
			makeRow(widget.NewLabel("Advanced Tests"), container.NewHBox(checkTLS, checkHTTP)),
			makeRow(lblIPCount, inputIPCount),
			makeRow(lblScanMode, selectScanMode),
			makeRow(lblIPs, ipContainer),
			makeRow(lblCustomIPs, inputCustomIPs),
			makeRow(lblXrayConfig, inputXrayConfig),
			makeRow(lblXrayThreads, inputXrayThreads),
			makeRow(lblXrayCore, checkXrayBox),
		}
		settingsContainer.Refresh()
	}

	if settingsBorder != nil {
		if isDarkTheme {
			settingsBorder.StrokeColor = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
		} else {
			settingsBorder.StrokeColor = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
		}
		settingsBorder.Refresh()
	}

	if tableBorder != nil {
		if isDarkTheme {
			tableBorder.StrokeColor = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
		} else {
			tableBorder.StrokeColor = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
		}
		tableBorder.Refresh()
	}

	if bottomBorder != nil {
		if isDarkTheme {
			bottomBorder.StrokeColor = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
		} else {
			bottomBorder.StrokeColor = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
		}
		bottomBorder.Refresh()
	}

	if fyneWin != nil {
		fyneWin.SetTitle("CF Scanner")
	}
	if !isRunning {
		if btnStart != nil {
			btnStart.SetText(T("start_test"))
		}
	} else {
		if btnStart != nil {
			btnStart.SetText(T("stop_test"))
		}
	}

	if checkTLS != nil {
		checkTLS.SetText(T("test_tls"))
	}
	if checkHTTP != nil {
		checkHTTP.SetText(T("test_http"))
	}

	if btnTabSettings != nil {
		btnTabSettings.SetText(T("settings"))
	}
	if btnTabResults != nil {
		btnTabResults.SetText(T("results_tab"))
	}
	if btnTabExport != nil {
		btnTabExport.SetText(T("export_tab"))
	}
	if btnTabTest != nil {
		btnTabTest.SetText(T("test_tab"))
	}

	if lblThreads != nil {
		lblThreads.SetText(T("threads"))
	}
	if lblPingCount != nil {
		lblPingCount.SetText(T("ping_count"))
	}
	if lblPort != nil {
		lblPort.SetText(T("port"))
	}
	if lblSNI != nil {
		lblSNI.SetText(T("sni"))
	}
	if lblMode != nil {
		lblMode.SetText(T("mode"))
	}
	if lblIPCount != nil {
		lblIPCount.SetText(T("ip_count"))
	}
	if lblScanMode != nil {
		lblScanMode.SetText(T("scan_mode"))
	}
	if selectScanMode != nil {
		currentScanMode := selectScanMode.Selected
		selectScanMode.Options = []string{T("scan_random"), T("scan_sequential")}
		if currentScanMode == "Random" || currentScanMode == "رندوم" {
			selectScanMode.SetSelected(T("scan_random"))
		} else if currentScanMode == "Sequential" || currentScanMode == "به ترتیب" {
			selectScanMode.SetSelected(T("scan_sequential"))
		}
	}
	if lblIPs != nil {
		lblIPs.SetText(T("select_ips"))
	}
	if lblCustomIPs != nil {
		lblCustomIPs.SetText(T("custom_ips"))
	}

	if btnSelectAll != nil {
		btnSelectAll.SetText(T("select_all"))
	}
	if btnClearAll != nil {
		btnClearAll.SetText(T("clear_all"))
	}
	if btnFetchIPv4 != nil {
		btnFetchIPv4.SetText(T("fetch_ipv4"))
	}

	if lblXrayConfig != nil {
		lblXrayConfig.SetText(T("xray_config"))
	}
	if btnCheckXray != nil {
		btnCheckXray.SetText(T("xray_check"))
	}

	if lblExportBaseURI != nil {
		lblExportBaseURI.SetText(T("export_base_uri"))
	}
	if lblExportCount != nil {
		lblExportCount.SetText(T("export_count"))
	}
	if btnExportGenerate != nil {
		btnExportGenerate.SetText(T("export_btn"))
	}

	if lblXrayThreads != nil {
		lblXrayThreads.SetText(T("xray_threads"))
	}
	if lblXrayCore != nil {
		lblXrayCore.SetText(T("xray_core"))
	}

	if btnCopyAlive != nil {
		btnCopyAlive.SetText(T("copy_alive"))
	}
	if btnExportAlive != nil {
		btnExportAlive.SetText(T("export_alive"))
	}

	if testHeaderBtns[0] != nil {
		testHeaderBtns[0].SetText(T("col_ip"))
		testHeaderBtns[1].SetText(T("col_config"))
		testHeaderBtns[2].SetText(T("col_score"))
		testHeaderBtns[3].SetText(T("col_real_delay"))
		testHeaderBtns[4].SetText(T("col_copy"))
	}

	if headerBtns[0] != nil {
		headerBtns[0].SetText(T("col_ip"))
		headerBtns[1].SetText(T("col_sent"))
		headerBtns[2].SetText(T("col_recv"))
		headerBtns[3].SetText(T("col_loss"))
		headerBtns[4].SetText(T("col_delay"))
		headerBtns[5].SetText(T("col_jitter"))
		headerBtns[6].SetText(T("col_score"))
		headerBtns[7].SetText(T("col_colo"))
	}
}

func fetchIPv4(checkGroup *widget.CheckGroup) {
	statusBinding.Set(T("fetch_ipv4") + "...")
	statusIcon.SetResource(theme.DownloadIcon())
	btnStart.Disable()

	resp, err := http.Get("https://www.cloudflare.com/ips-v4")
	if err != nil {
		statusBinding.Set(T("status_error"))
		statusIcon.SetResource(theme.ErrorIcon())
		btnStart.Enable()
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		statusBinding.Set(T("status_error"))
		statusIcon.SetResource(theme.ErrorIcon())
		btnStart.Enable()
		return
	}

	lines := strings.Split(string(body), "\n")
	var newRanges []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			newRanges = append(newRanges, line)
		}
	}

	if len(newRanges) > 0 {
		defaultIPRanges = newRanges
		checkGroup.Options = defaultIPRanges
		checkGroup.SetSelected(defaultIPRanges)
		checkGroup.Refresh()
		statusBinding.Set(T("status_done"))
		statusIcon.SetResource(theme.ConfirmIcon())
	} else {
		statusBinding.Set(T("status_error"))
		statusIcon.SetResource(theme.ErrorIcon())
	}
	btnStart.Enable()
}


func startTest() {
	setStopButton()
	statusBinding.Set(T("status_pinging"))
	statusIcon.SetResource(theme.SearchIcon())
	progressBinding.Set(0)

	switchTab(1)

	utils.CancelCtx, utils.CancelFunc = context.WithCancel(context.Background())
	resultsData = make([]utils.CloudflareIPData, 0)
	refreshDataVBox()

	task.Routines, _ = strconv.Atoi(inputThreads.Text)
	
	sni := strings.TrimSpace(inputSNI.Text)
	if sni != "" {
		task.SNI = sni
	} else {
		task.SNI = "speed.cloudflare.com"
	}
	
	task.PingTimes, _ = strconv.Atoi(inputPingCount.Text)
	task.TCPPort, _ = strconv.Atoi(inputPort.Text)
	task.TestTLS = checkTLS.Checked
	task.TestHTTP = checkHTTP.Checked
	task.IPsPerRange, _ = strconv.Atoi(inputIPCount.Text)

	task.IPText = ""
	for i, r := range selectedIPRanges {
		task.IPText += r
		if i < len(selectedIPRanges)-1 {
			task.IPText += ","
		}
	}

	customText := strings.TrimSpace(inputCustomIPs.Text)
	if customText != "" {
		customLines := strings.Split(customText, "\n")
		for _, line := range customLines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				if task.IPText != "" {
					task.IPText += ","
				}
				task.IPText += line
			}
		}
	}

	task.IPFile = ""

	go func() {
		defer resetStartButton()

		task.InitRandSeed()

		pingSet := task.NewPing().Run().FilterDelay().FilterLossRate()

		if utils.CancelCtx.Err() != nil {
			statusBinding.Set(T("test_stopped"))
			statusIcon.SetResource(theme.CancelIcon())
			return
		}

		if len(pingSet) == 0 {
			statusBinding.Set(T("status_error"))
			statusIcon.SetResource(theme.ErrorIcon())
			dialog.ShowInformation(T("error_title"), T("no_ips_alive"), fyneWin)
			return
		}

		resultsData = pingSet
		refreshDataVBox()

		statusBinding.Set(T("status_done"))
		statusIcon.SetResource(theme.ConfirmIcon())

		// Populate Xray Table
		xrayTableData = []XrayResult{}
		configUri := strings.TrimSpace(inputXrayConfig.Text)

		for i := 0; i < len(pingSet); i++ {
			ipData := pingSet[i]
			ipStr := ipData.IP.String()

			genConfig := ""
			if configUri != "" {
				genConfig = ReplaceIPInURI(configUri, ipStr)
			}

			xrayTableData = append(xrayTableData, XrayResult{
				IP:     ipStr,
				Config: genConfig,
				Score:  ipData.Score,
				Delay:  0, // Not tested yet
			})
		}
		if xrayVBox != nil {
			refreshXrayVBox()
		}

		utils.ExportCsv(pingSet)
	}()
}

func buildExportUI() {
	lblExportBaseURI = widget.NewLabel(T("export_base_uri"))
	inputExportBaseURI = widget.NewMultiLineEntry()
	inputExportBaseURI.SetPlaceHolder("vless://...\ntrojan://...")

	lblExportCount = widget.NewLabel(T("export_count"))
	inputExportCount = widget.NewEntry()
	inputExportCount.SetText("5")

	outputExportConfigs = widget.NewMultiLineEntry()

	btnExportGenerate = widget.NewButtonWithIcon(T("export_btn"), theme.DocumentCreateIcon(), func() {
		generateConfigs()
	})
	btnExportGenerate.Importance = widget.HighImportance

	btnCopy := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		if outputExportConfigs.Text != "" && fyneWin != nil {
			fyneWin.Clipboard().SetContent(outputExportConfigs.Text)
			statusBinding.Set(T("export_copied"))
			statusIcon.SetResource(theme.ConfirmIcon())
		}
	})

	topContent := container.NewBorder(
		lblExportBaseURI,
		container.NewHBox(lblExportCount, inputExportCount, btnExportGenerate),
		nil, nil,
		inputExportBaseURI,
	)

	copyContainer := container.NewHBox(layout.NewSpacer(), btnCopy)

	mainBorder := container.NewBorder(topContent, copyContainer, nil, nil, outputExportConfigs)

	exportContent = container.NewPadded(mainBorder)
}

func generateConfigs() {
	count, _ := strconv.Atoi(inputExportCount.Text)
	if count <= 0 {
		count = 5
	}
	if count > len(resultsData) {
		count = len(resultsData)
	}

	baseURIsText := strings.TrimSpace(inputExportBaseURI.Text)
	if baseURIsText == "" || count == 0 {
		return
	}

	lines := strings.Split(baseURIsText, "\n")
	var baseURIs []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			baseURIs = append(baseURIs, l)
		}
	}

	var out []string

	configCounter := 1
	for i := 0; i < count; i++ {
		data := resultsData[i]
		ipStr := data.IP.String()
		if strings.Contains(ipStr, ":") {
			ipStr = "[" + ipStr + "]"
		}

		for _, baseURI := range baseURIs {
			u, err := url.Parse(baseURI)
			if err == nil && u.Scheme != "" && u.Host != "" {
				uCopy, _ := url.Parse(baseURI)
				port := uCopy.Port()
				if port != "" {
					uCopy.Host = ipStr + ":" + port
				} else {
					uCopy.Host = ipStr
				}

				uCopy.Fragment = fmt.Sprintf("CloudFlare%d", configCounter)
				configCounter++

				outString := uCopy.String()
				out = append(out, outString)
			} else {
				parts := strings.SplitN(baseURI, "@", 2)
				newURI := baseURI
				if len(parts) == 2 {
					hostEnd := strings.IndexAny(parts[1], ":?/#")
					if hostEnd == -1 {
						hostEnd = len(parts[1])
					}
					newHostPart := ipStr + parts[1][hostEnd:]
					newURI = parts[0] + "@" + newHostPart
				} else {
					newURI = fmt.Sprintf("%s -> %s", baseURI, ipStr)
				}

				if strings.Contains(newURI, "#") {
					newURI = newURI[:strings.Index(newURI, "#")]
				}
				newURI += fmt.Sprintf("#CloudFlare%d", configCounter)
				configCounter++

				out = append(out, newURI)
			}
		}
	}

	outputExportConfigs.SetText(strings.Join(out, "\n"))

	if inputTestConfigs != nil {
		inputTestConfigs.SetText(strings.Join(out, "\n"))
	}
}

func refreshXrayVBox() {
	if xrayVBox == nil {
		return
	}

	// Recycle existing objects if length matches (avoids lag during test and sorting)
	if len(xrayVBox.Objects) > 0 && len(xrayVBox.Objects) == len(xrayTableData)+1 {
		for id, data := range xrayTableData {
			rowContainer := xrayVBox.Objects[id+1].(*fyne.Container)
			rowContent := rowContainer.Objects[1].(*fyne.Container)

			for i := 0; i < 5; i++ {
				cellStack := rowContent.Objects[i].(*fyne.Container)
				cellBg := cellStack.Objects[0].(*canvas.Rectangle)
				cellBg.FillColor = color.Transparent

				if i == 4 {
					// Copy button - update the bound config string if needed
					btn := cellStack.Objects[1].(*fyne.Container).Objects[0].(*widget.Button)
					btn.OnTapped = func(cfg string) func() {
						return func() {
							if fyneWin != nil {
								fyneWin.Clipboard().SetContent(cfg)
								statusBinding.Set(T("export_copied"))
								statusIcon.SetResource(theme.ConfirmIcon())
							}
						}
					}(data.Config)
				} else {
					lbl := cellStack.Objects[1].(*widget.Label)
					switch i {
					case 0:
						lbl.SetText(data.IP + "‎")
					case 1:
						lbl.SetText(data.Config + "‎")
					case 2:
						lbl.SetText(strconv.FormatFloat(data.Score, 'f', 2, 64) + "‎")
					case 3:
						if data.Delay == 0 {
							lbl.SetText("-")
						} else if data.Delay == -1 {
							lbl.SetText("Error")
							cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
						} else {
							lbl.SetText(strconv.Itoa(data.Delay) + " ms‎")
							if data.Delay > 0 && data.Delay < 500 {
								cellBg.FillColor = color.NRGBA{R: 100, G: 255, B: 100, A: 50}
							} else {
								cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
							}
						}
					}
				}
			}
		}
		xrayVBox.Refresh()
		return
	}

	xrayVBox.Objects = nil

	headers := []string{T("col_ip"), T("col_config"), T("col_score"), T("col_real_delay"), T("col_copy")}
	colWidths := []float32{140, 350, 140, 140, 140}

	headerCells := make([]fyne.CanvasObject, 5)
	for i, text := range headers {
		colIndex := i
		btn := widget.NewButton(text, func() {
			if colIndex == 0 || colIndex == 2 || colIndex == 3 {
				sortXrayData(colIndex)
				refreshXrayVBox()
			}
		})
		btn.Importance = widget.LowImportance
		testHeaderBtns[i] = btn
		spacer := canvas.NewRectangle(color.Transparent)
		spacer.SetMinSize(fyne.NewSize(colWidths[i], 0))
		headerCells[i] = container.NewStack(spacer, btn)
	}
	headerContainer := container.New(&tableRowLayout{widths: colWidths}, headerCells...)
	xrayVBox.Add(headerContainer)

	for id, data := range xrayTableData {
		rowBg := canvas.NewRectangle(color.Transparent)
		if id%2 == 1 {
			rowBg.FillColor = color.NRGBA{R: 128, G: 128, B: 128, A: 20}
		}

		cells := make([]fyne.CanvasObject, 5)
		for i := 0; i < 5; i++ {
			cellBg := canvas.NewRectangle(color.Transparent)
			
			if i == 4 {
				btn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), nil)
				btn.Importance = widget.LowImportance
				btn.OnTapped = func(cfg string) func() {
					return func() {
						if fyneWin != nil {
							fyneWin.Clipboard().SetContent(cfg)
							statusBinding.Set(T("export_copied"))
							statusIcon.SetResource(theme.ConfirmIcon())
						}
					}
				}(data.Config)
				cells[i] = container.NewStack(cellBg, container.NewPadded(btn))
			} else {
				lbl := widget.NewLabel("")
				lbl.Alignment = fyne.TextAlignCenter
				if i == 1 {
					lbl.Truncation = fyne.TextTruncateEllipsis
				}
				
				switch i {
				case 0:
					lbl.SetText(data.IP + "‎")
				case 1:
					lbl.SetText(data.Config + "‎")
				case 2:
					lbl.SetText(strconv.FormatFloat(data.Score, 'f', 2, 64) + "‎")
				case 3:
					if data.Delay == 0 {
						lbl.SetText("-")
					} else if data.Delay == -1 {
						lbl.SetText("Error")
						cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
					} else {
						lbl.SetText(strconv.Itoa(data.Delay) + " ms‎")
						if data.Delay > 0 && data.Delay < 500 {
							cellBg.FillColor = color.NRGBA{R: 100, G: 255, B: 100, A: 50}
						} else {
							cellBg.FillColor = color.NRGBA{R: 255, G: 100, B: 100, A: 50}
						}
					}
				}
				cells[i] = container.NewStack(cellBg, lbl)
			}
		}
		rowContent := container.New(&tableRowLayout{widths: colWidths}, cells...)
		xrayVBox.Add(container.NewStack(rowBg, rowContent))
	}
	xrayVBox.Refresh()
}

func buildTestUI() {
	xrayVBox = container.NewVBox()
	refreshXrayVBox()

	tableBorder := canvas.NewRectangle(color.Transparent)
	tableBorder.StrokeColor = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
	tableBorder.StrokeWidth = 1
	tableBorder.CornerRadius = 8

	xrayScroll = container.NewScroll(xrayVBox)
	paddedList := withPadding(xrayScroll, 12)

	tableCard := container.NewStack(paddedList, tableBorder)
	btnCopyAlive = widget.NewButtonWithIcon(T("copy_alive"), theme.ContentCopyIcon(), func() {
		var alive []string
		for _, v := range xrayTableData {
			if v.Delay > 0 {
				alive = append(alive, v.Config)
			}
		}
		if len(alive) > 0 && fyneWin != nil {
			fyneWin.Clipboard().SetContent(strings.Join(alive, "\n"))
			dialog.ShowInformation("Success", fmt.Sprintf("Copied %d configs!", len(alive)), fyneWin)
		}
	})

	btnExportAlive = widget.NewButtonWithIcon(T("export_alive"), theme.DocumentSaveIcon(), func() {
		var alive []string
		for _, v := range xrayTableData {
			if v.Delay > 0 {
				alive = append(alive, v.Config)
			}
		}
		if len(alive) > 0 {
			err := os.WriteFile("xray_alive.txt", []byte(strings.Join(alive, "\n")), 0644)
			if err == nil {
				dialog.ShowInformation("Success", fmt.Sprintf("Saved %d configs to xray_alive.txt!", len(alive)), fyneWin)
			}
		}
	})

	actionButtons := container.NewGridWithColumns(2, btnCopyAlive, btnExportAlive)

	actionsBorder := canvas.NewRectangle(color.Transparent)
	actionsBorder.StrokeWidth = 1
	actionsBorder.CornerRadius = 8
	if isDarkTheme {
		actionsBorder.StrokeColor = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
	} else {
		actionsBorder.StrokeColor = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	}

	paddedActions := withPadding(actionButtons, 12)
	actionsCard := container.NewStack(paddedActions, actionsBorder)

	testContent = container.NewBorder(nil, container.NewPadded(actionsCard), nil, nil, container.NewPadded(tableCard))
}

func ReplaceIPInURI(uri, newIP string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}

	q := u.Query()
	oldHost := u.Hostname()
	if q.Get("sni") == "" {
		q.Set("sni", oldHost)
		u.RawQuery = q.Encode()
	}

	port := u.Port()
	if port != "" {
		u.Host = newIP + ":" + port
	} else {
		u.Host = newIP
	}
	return u.String()
}

func startXrayTest() {
	if inputXrayConfig.Text == "" {
		msg := "Error: Xray Config is empty!"
		if currentLang == LangFA {
			msg = "خطا: کانفیگ ایکس ری (vless/trojan) وارد نشده است!"
		}
		dialog.ShowInformation(T("error_title"), msg, fyneWin)
		return
	}
	if !task.CheckXrayCore() {
		msg := "Error: Xray core not found! Please download it from Settings."
		if currentLang == LangFA {
			msg = "خطا: هسته ایکس ری یافت نشد! لطفاً از بخش تنظیمات آن را بررسی و نصب کنید."
		}
		dialog.ShowInformation(T("error_title"), msg, fyneWin)
		return
	}
	if len(xrayTableData) == 0 {
		if currentLang == LangFA {
			statusBinding.Set("خطا: لیست آی‌پی خالی است، اول تست اصلی را بزنید!")
		} else {
			statusBinding.Set("Error: Run Cloudflare test first!")
		}
		statusIcon.SetResource(theme.ErrorIcon())
		return
	}

	setStopButton()

	statusBinding.Set("Testing Real Delay with Xray...")
	statusIcon.SetResource(theme.SearchIcon())
	progressBinding.Set(0)

	total := len(xrayTableData)
	configUri := strings.TrimSpace(inputXrayConfig.Text)

	threads, err := strconv.Atoi(inputXrayThreads.Text)
	if err != nil || threads < 1 {
		threads = 5
	}

	sem := make(chan struct{}, threads)
	var wg sync.WaitGroup
	var finishedCount int32

	for i := 0; i < total; i++ {
		if !isRunning {
			break
		} // User clicked stop

		// Generate config dynamically based on current input box
		xrayTableData[i].Config = ReplaceIPInURI(configUri, xrayTableData[i].IP)

		wg.Add(1)
		sem <- struct{}{}

		go func(index int) {
			defer wg.Done()
			defer func() { <-sem }()

			if !isRunning {
				return
			}

			var delay int
			var testErr error
			for attempt := 0; attempt < 2; attempt++ {
				delay, testErr = task.TestRealDelay(xrayTableData[index].Config)
				if testErr == nil {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			if testErr != nil {
				delay = -1
			}

			xrayTableData[index].Delay = delay
			xrayTableData[index].Delay = delay
			if xrayVBox != nil {
				refreshXrayVBox()
			}

			current := atomic.AddInt32(&finishedCount, 1)
			progressBinding.Set(float64(current) / float64(total))
			statusBinding.Set(fmt.Sprintf("Xray Testing... (%d/%d)", current, total))
		}(i)
	}

	wg.Wait()

	statusBinding.Set(T("status_done"))
	statusIcon.SetResource(theme.InfoIcon())
	progressBinding.Set(1.0)

	resetStartButton()
}
