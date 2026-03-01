package chatlog

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/TE0dollary/chatlog-bot/internal/chatlog/ctx"
	"github.com/TE0dollary/chatlog-bot/internal/ui/footer"
	"github.com/TE0dollary/chatlog-bot/internal/ui/form"
	"github.com/TE0dollary/chatlog-bot/internal/ui/help"
	"github.com/TE0dollary/chatlog-bot/internal/ui/infobar"
	"github.com/TE0dollary/chatlog-bot/internal/ui/logview"
	"github.com/TE0dollary/chatlog-bot/internal/ui/menu"
	"github.com/TE0dollary/chatlog-bot/internal/wechat/model"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	// RefreshInterval 控制 TUI 信息栏的自动刷新频率（1 秒）。
	RefreshInterval = 1000 * time.Millisecond
)

// App 是基于 tview 框架的 TUI 终端界面。
// 布局从上到下：InfoBar（状态信息表格）→ TabPages（主菜单/帮助页）→ LogView（日志面板）→ Footer（底部栏）。
//
// 交互方式：
//   - 上下键：选择菜单项
//   - Enter：确认选择
//   - 左右键：切换标签页
//   - ESC：从子菜单返回主页
//   - Ctrl+C：退出程序
type App struct {
	*tview.Application

	ctx         *ctx.Context
	m           *Manager
	stopRefresh chan struct{} // 关闭时通知刷新 goroutine 退出

	// 页面层级：mainPages 管理主视图和子菜单/模态框的叠加
	mainPages *tview.Pages
	infoBar   *infobar.InfoBar // 顶部状态信息栏（账号、密钥、目录、服务状态等）
	tabPages  *tview.Pages     // 标签页容器（主菜单 / 帮助）
	logView   *logview.LogView // 底部日志滚动面板
	footer    *footer.Footer   // 底部提示栏

	// 标签页
	menu      *menu.Menu // 主菜单（获取密钥、解密、HTTP 服务、自动解密、设置、切换账号、退出）
	help      *help.Help // 帮助页
	activeTab int        // 当前活跃标签页索引
	tabCount  int        // 标签页总数
}

// NewApp 创建并初始化 TUI 应用，设置菜单项和初始状态。
// lv 为可选的日志面板；传入 nil 时不显示日志区域。
func NewApp(ctx *ctx.Context, m *Manager, lv *logview.LogView) *App {
	app := &App{
		ctx:         ctx,
		m:           m,
		Application: tview.NewApplication(),
		mainPages:   tview.NewPages(),
		infoBar:     infobar.New(),
		tabPages:    tview.NewPages(),
		logView:     lv,
		footer:      footer.New(),
		menu:        menu.New("主菜单"),
		help:        help.New(),
	}

	app.initMenu()
	app.updateMenuItemsState()
	return app
}

// logViewHeight 日志面板固定高度（行数）。
const logViewHeight = 12

// Run 启动 TUI 界面（阻塞直到用户退出）。
// 同时启动后台 goroutine 每秒刷新信息栏数据。
func (a *App) Run() error {

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.infoBar, infobar.InfoBarViewHeight, 0, false).
		AddItem(a.tabPages, 0, 1, true)

	if a.logView != nil {
		flex.AddItem(a.logView, logViewHeight, 0, false)

		// 注入重绘函数：日志写入后自动滚动到末尾
		a.logView.SetRedrawFunc(func() {
			a.QueueUpdateDraw(func() {
				a.logView.ScrollToEnd()
			})
		})
	}

	flex.AddItem(a.footer, 1, 1, false)

	a.mainPages.AddPage("main", flex, true, true)

	a.tabPages.
		AddPage("0", a.menu, true, true).
		AddPage("1", a.help, true, false)
	a.tabCount = 2

	a.SetInputCapture(a.inputCapture)

	go a.refresh()

	if err := a.SetRoot(a.mainPages, true).EnableMouse(false).Run(); err != nil {
		return err
	}

	return nil
}

func (a *App) Stop() {
	// 添加一个通道用于停止刷新 goroutine
	if a.stopRefresh != nil {
		close(a.stopRefresh)
	}
	a.Application.Stop()
}

func (a *App) updateMenuItemsState() {
	// 查找并更新自动解密菜单项
	for _, item := range a.menu.GetItems() {
		// 更新自动解密菜单项
		if item.Index == 6 {
			if a.ctx.GetAutoDecrypt() {
				item.Name = "停止自动解密"
				item.Description = "停止监控数据目录更新，不再自动解密新增数据"
			} else {
				item.Name = "开启自动解密"
				item.Description = "监控数据目录更新，自动解密新增数据"
			}
		}

		// 更新HTTP服务菜单项
		if item.Index == 5 {
			if a.ctx.GetHTTPEnabled() {
				item.Name = "停止 HTTP 服务"
				item.Description = "停止本地 HTTP & MCP 服务器"
			} else {
				item.Name = "启动 HTTP 服务"
				item.Description = "启动本地 HTTP & MCP 服务器"
			}
		}
	}
}

func (a *App) switchTab(step int) {
	index := (a.activeTab + step) % a.tabCount
	if index < 0 {
		index = a.tabCount - 1
	}
	a.activeTab = index
	a.tabPages.SwitchToPage(fmt.Sprint(a.activeTab))
}

// refresh 后台每秒刷新信息栏的各项状态数据（账号、密钥、目录大小、服务状态等）。
func (a *App) refresh() {
	tick := time.NewTicker(RefreshInterval)
	defer tick.Stop()

	for {
		select {
		case <-a.stopRefresh:
			return
		case <-tick.C:
			if a.ctx.GetAutoDecrypt() || a.ctx.GetHTTPEnabled() {
				a.m.RefreshSession()
			}
			a.infoBar.UpdateAccount(a.ctx.GetProcess().Name)
			a.infoBar.UpdateBasicInfo(int(a.ctx.GetProcess().PID), a.ctx.GetProcess().FullVersion, a.ctx.GetProcess().ExePath)
			a.infoBar.UpdateStatus(a.ctx.GetProcess().Status)
			a.infoBar.UpdateDataKey(a.ctx.GetDataKey())
			a.infoBar.UpdateImageKey(a.ctx.GetImgKey())
			a.infoBar.UpdatePlatform(a.ctx.GetProcess().Platform)
			a.infoBar.UpdateDataUsageDir(a.ctx.GetDataUsage(), a.ctx.GetDataDir())
			a.infoBar.UpdateWorkUsageDir(a.ctx.GetWorkUsage(), a.ctx.GetWorkDir())
			if a.ctx.GetLastSession().Unix() > 1000000000 {
				a.infoBar.UpdateSession(a.ctx.GetLastSession().Format("2006-01-02 15:04:05"))
			}
			if a.ctx.GetHTTPEnabled() {
				a.infoBar.UpdateHTTPServer(fmt.Sprintf("[green][已启动][white] [%s]", a.ctx.GetHTTPAddr()))
			} else {
				a.infoBar.UpdateHTTPServer("[未启动]")
			}
			if a.ctx.GetAutoDecrypt() {
				a.infoBar.UpdateAutoDecrypt("[green][已开启][white]")
			} else {
				a.infoBar.UpdateAutoDecrypt("[未开启]")
			}
			if a.ctx.GetProcess() != nil {
				a.infoBar.UpdateDerivedKeyCount(len(a.ctx.GetDerivedKeyMap()))
			} else {
				a.infoBar.UpdateDerivedKeyCount(0)
			}

			a.Draw()
		}
	}
}

func (a *App) inputCapture(event *tcell.EventKey) *tcell.EventKey {

	// 如果当前页面不是主页面，ESC 键返回主页面
	if a.mainPages.HasPage("submenu") && event.Key() == tcell.KeyEscape {
		a.mainPages.RemovePage("submenu")
		a.mainPages.SwitchToPage("main")
		return nil
	}

	if a.tabPages.HasFocus() {
		switch event.Key() {
		case tcell.KeyLeft:
			a.switchTab(-1)
			return nil
		case tcell.KeyRight:
			a.switchTab(1)
			return nil
		}
	}

	switch event.Key() {
	case tcell.KeyCtrlC:
		a.Stop()
	}

	return event
}

// initMenu 初始化主菜单的 7 个菜单项及其回调逻辑。
func (a *App) initMenu() {
	getDataKey := &menu.Item{
		Index:       2,
		Name:        "获取密钥",
		Description: "从进程获取数据密钥 & 图片密钥",
		Selected: func(i *menu.Item) {
			modal := tview.NewModal()
			if runtime.GOOS == "darwin" {
				modal.SetText("获取密钥中...\n预计需要 20 秒左右的时间，期间微信会卡住，请耐心等待")
			} else {
				modal.SetText("获取密钥中...")
			}
			a.mainPages.AddPage("modal", modal, true, true)
			a.SetFocus(modal)

			go func() {
				err := a.m.GetDataKey()

				// 在主线程中更新UI
				a.QueueUpdateDraw(func() {
					if err != nil {
						// 解密失败
						modal.SetText("获取密钥失败: " + err.Error())
					} else {
						// 解密成功
						modal.SetText("获取密钥成功")
					}

					// 添加确认按钮
					modal.AddButtons([]string{"OK"})
					modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						a.mainPages.RemovePage("modal")
					})
					a.SetFocus(modal)
				})
			}()
		},
	}

	decryptData := &menu.Item{
		Index:       4,
		Name:        "解密数据",
		Description: "解密数据文件",
		Selected: func(i *menu.Item) {
			// 创建一个没有按钮的模态框，显示"解密中..."
			modal := tview.NewModal().
				SetText("解密中...")

			a.mainPages.AddPage("modal", modal, true, true)
			a.SetFocus(modal)

			// 在后台执行解密操作
			go func() {
				// 执行解密
				err := a.m.DecryptDBFiles()

				// 在主线程中更新UI
				a.QueueUpdateDraw(func() {
					if err != nil {
						// 解密失败
						modal.SetText("解密失败: " + err.Error())
					} else {
						// 解密成功
						modal.SetText("解密数据成功")
					}

					// 添加确认按钮
					modal.AddButtons([]string{"OK"})
					modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						a.mainPages.RemovePage("modal")
					})
					a.SetFocus(modal)
				})
			}()
		},
	}

	httpServer := &menu.Item{
		Index:       5,
		Name:        "启动 HTTP 服务",
		Description: "启动本地 HTTP & MCP 服务器",
		Selected: func(i *menu.Item) {
			modal := tview.NewModal()

			// 根据当前服务状态执行不同操作
			if !a.ctx.GetHTTPEnabled() {
				// HTTP 服务未启动，启动服务
				modal.SetText("正在启动 HTTP 服务...")
				a.mainPages.AddPage("modal", modal, true, true)
				a.SetFocus(modal)

				// 在后台启动服务
				go func() {
					err := a.m.StartService()

					// 在主线程中更新UI
					a.QueueUpdateDraw(func() {
						if err != nil {
							// 启动失败
							modal.SetText("启动 HTTP 服务失败: " + err.Error())
						} else {
							// 启动成功
							modal.SetText("已启动 HTTP 服务")
						}

						// 更改菜单项名称
						a.updateMenuItemsState()

						// 添加确认按钮
						modal.AddButtons([]string{"OK"})
						modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							a.mainPages.RemovePage("modal")
						})
						a.SetFocus(modal)
					})
				}()
			} else {
				// HTTP 服务已启动，停止服务
				modal.SetText("正在停止 HTTP 服务...")
				a.mainPages.AddPage("modal", modal, true, true)
				a.SetFocus(modal)

				// 在后台停止服务
				go func() {
					err := a.m.StopService()

					// 在主线程中更新UI
					a.QueueUpdateDraw(func() {
						if err != nil {
							// 停止失败
							modal.SetText("停止 HTTP 服务失败: " + err.Error())
						} else {
							// 停止成功
							modal.SetText("已停止 HTTP 服务")
						}

						// 更改菜单项名称
						a.updateMenuItemsState()

						// 添加确认按钮
						modal.AddButtons([]string{"OK"})
						modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							a.mainPages.RemovePage("modal")
						})
						a.SetFocus(modal)
					})
				}()
			}
		},
	}

	autoDecrypt := &menu.Item{
		Index:       6,
		Name:        "开启自动解密",
		Description: "自动解密新增的数据文件",
		Selected: func(i *menu.Item) {
			modal := tview.NewModal()

			// 根据当前自动解密状态执行不同操作
			if !a.ctx.GetAutoDecrypt() {
				// 自动解密未开启，开启自动解密
				modal.SetText("正在开启自动解密...")
				a.mainPages.AddPage("modal", modal, true, true)
				a.SetFocus(modal)

				// 在后台开启自动解密
				go func() {
					err := a.m.StartAutoDecrypt()

					// 在主线程中更新UI
					a.QueueUpdateDraw(func() {
						if err != nil {
							// 开启失败
							modal.SetText("开启自动解密失败: " + err.Error())
						} else {
							// 开启成功
							modal.SetText("已开启自动解密")
						}

						// 更改菜单项名称
						a.updateMenuItemsState()

						// 添加确认按钮
						modal.AddButtons([]string{"OK"})
						modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							a.mainPages.RemovePage("modal")
						})
						a.SetFocus(modal)
					})
				}()
			} else {
				// 自动解密已开启，停止自动解密
				modal.SetText("正在停止自动解密...")
				a.mainPages.AddPage("modal", modal, true, true)
				a.SetFocus(modal)

				// 在后台停止自动解密
				go func() {
					err := a.m.StopAutoDecrypt()

					// 在主线程中更新UI
					a.QueueUpdateDraw(func() {
						if err != nil {
							// 停止失败
							modal.SetText("停止自动解密失败: " + err.Error())
						} else {
							// 停止成功
							modal.SetText("已停止自动解密")
						}

						// 更改菜单项名称
						a.updateMenuItemsState()

						// 添加确认按钮
						modal.AddButtons([]string{"OK"})
						modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
							a.mainPages.RemovePage("modal")
						})
						a.SetFocus(modal)
					})
				}()
			}
		},
	}

	setting := &menu.Item{
		Index:       7,
		Name:        "设置",
		Description: "设置应用程序选项",
		Selected:    a.settingSelected,
	}

	selectAccount := &menu.Item{
		Index:       8,
		Name:        "切换账号",
		Description: "切换当前操作的账号，可以选择进程或历史账号",
		Selected:    a.selectAccountSelected,
	}

	viewDerivedKeys := &menu.Item{
		Index:       3,
		Name:        "查看派生密钥",
		Description: "展示所有已获取的派生密钥列表",
		Selected: func(i *menu.Item) {
			a.showDerivedKeys()
		},
	}

	a.menu.AddItem(getDataKey)
	a.menu.AddItem(decryptData)
	a.menu.AddItem(httpServer)
	a.menu.AddItem(autoDecrypt)
	a.menu.AddItem(setting)
	a.menu.AddItem(selectAccount)

	a.menu.AddItem(&menu.Item{
		Index:       9,
		Name:        "退出",
		Description: "退出程序",
		Selected: func(i *menu.Item) {
			a.Stop()
		},
	})
	a.menu.AddItem(viewDerivedKeys)
}

// settingItem 表示一个设置项
type settingItem struct {
	name        string
	description string
	action      func()
}

func (a *App) settingSelected(i *menu.Item) {

	settings := []settingItem{
		{
			name:        "设置 HTTP 服务地址",
			description: "配置 HTTP 服务监听的地址",
			action:      a.settingHTTPPort,
		},
		{
			name:        "设置工作目录",
			description: "配置数据解密后的存储目录",
			action:      a.settingWorkDir,
		},
		{
			name:        "设置数据密钥",
			description: "配置数据解密密钥",
			action:      a.settingDataKey,
		},
		{
			name:        "设置图片密钥",
			description: "配置图片解密密钥",
			action:      a.settingImgKey,
		},
		{
			name:        "设置数据目录",
			description: "配置微信数据文件所在目录",
			action:      a.settingDataDir,
		},
	}

	subMenu := menu.NewSubMenu("设置")
	for idx, setting := range settings {
		item := &menu.Item{
			Index:       idx + 1,
			Name:        setting.name,
			Description: setting.description,
			Selected: func(action func()) func(*menu.Item) {
				return func(*menu.Item) {
					action()
				}
			}(setting.action),
		}
		subMenu.AddItem(item)
	}

	a.mainPages.AddPage("submenu", subMenu, true, true)
	a.SetFocus(subMenu)
}

// settingHTTPPort 设置 HTTP 端口
func (a *App) settingHTTPPort() {
	// 使用我们的自定义表单组件
	formView := form.NewForm("设置 HTTP 地址")

	// 临时存储用户输入的值
	tempHTTPAddr := a.ctx.GetHTTPAddr()

	// 添加输入字段 - 不再直接设置HTTP地址，而是更新临时变量
	formView.AddInputField("地址", tempHTTPAddr, 0, nil, func(text string) {
		tempHTTPAddr = text // 只更新临时变量
	})

	// 添加按钮 - 点击保存时才设置HTTP地址
	formView.AddButton("保存", func() {
		a.m.SetHTTPAddr(tempHTTPAddr) // 在这里设置HTTP地址
		a.mainPages.RemovePage("submenu2")
		a.showInfo("HTTP 地址已设置为 " + a.ctx.GetHTTPAddr())
	})

	formView.AddButton("取消", func() {
		a.mainPages.RemovePage("submenu2")
	})

	a.mainPages.AddPage("submenu2", formView, true, true)
	a.SetFocus(formView)
}

// settingWorkDir 设置工作目录
func (a *App) settingWorkDir() {
	// 使用我们的自定义表单组件
	formView := form.NewForm("设置工作目录")

	// 临时存储用户输入的值
	tempWorkDir := a.ctx.GetWorkDir()

	// 添加输入字段 - 不再直接设置工作目录，而是更新临时变量
	formView.AddInputField("工作目录", tempWorkDir, 0, nil, func(text string) {
		tempWorkDir = text // 只更新临时变量
	})

	// 添加按钮 - 点击保存时才设置工作目录
	formView.AddButton("保存", func() {
		a.ctx.SetBaseWorkDir(tempWorkDir) // 在这里设置工作目录
		a.mainPages.RemovePage("submenu2")
		a.showInfo("工作目录已设置为 " + a.ctx.GetWorkDir())
	})

	formView.AddButton("取消", func() {
		a.mainPages.RemovePage("submenu2")
	})

	a.mainPages.AddPage("submenu2", formView, true, true)
	a.SetFocus(formView)
}

// settingDataKey 设置数据密钥
func (a *App) settingDataKey() {
	// 使用我们的自定义表单组件
	formView := form.NewForm("设置数据密钥")

	// 临时存储用户输入的值
	tempDataKey := a.ctx.GetDataKey()

	// 添加输入字段 - 不直接设置数据密钥，而是更新临时变量
	formView.AddInputField("数据密钥", tempDataKey, 0, nil, func(text string) {
		tempDataKey = text // 只更新临时变量
	})

	// 添加按钮 - 点击保存时才设置数据密钥
	formView.AddButton("保存", func() {
		a.ctx.SetDataKey(tempDataKey) // 设置数据密钥
		a.mainPages.RemovePage("submenu2")
		a.showInfo("数据密钥已设置")
	})

	formView.AddButton("取消", func() {
		a.mainPages.RemovePage("submenu2")
	})

	a.mainPages.AddPage("submenu2", formView, true, true)
	a.SetFocus(formView)
}

// settingImgKey 设置图片密钥 (ImgKey)
func (a *App) settingImgKey() {
	formView := form.NewForm("设置图片密钥")

	tempImgKey := a.ctx.GetImgKey()

	formView.AddInputField("图片密钥", tempImgKey, 0, nil, func(text string) {
		tempImgKey = text
	})

	formView.AddButton("保存", func() {
		a.ctx.SetImgKey(tempImgKey)
		a.mainPages.RemovePage("submenu2")
		a.showInfo("图片密钥已设置")
	})

	formView.AddButton("取消", func() {
		a.mainPages.RemovePage("submenu2")
	})

	a.mainPages.AddPage("submenu2", formView, true, true)
	a.SetFocus(formView)
}

// settingDataDir 设置数据目录
func (a *App) settingDataDir() {
	// 使用我们的自定义表单组件
	formView := form.NewForm("设置数据目录")

	// 临时存储用户输入的值
	tempDataDir := a.ctx.GetDataDirOverride()

	// 添加输入字段 - 不直接设置数据目录，而是更新临时变量
	formView.AddInputField("数据目录", tempDataDir, 0, nil, func(text string) {
		tempDataDir = text // 只更新临时变量
	})

	// 添加按钮 - 点击保存时才设置数据目录
	formView.AddButton("保存", func() {
		a.ctx.SetDataDir(tempDataDir) // 设置数据目录
		a.mainPages.RemovePage("submenu2")
		a.showInfo("数据目录已设置为 " + a.ctx.GetDataDir())
	})

	formView.AddButton("取消", func() {
		a.mainPages.RemovePage("submenu2")
	})

	a.mainPages.AddPage("submenu2", formView, true, true)
	a.SetFocus(formView)
}

// selectAccountSelected 处理切换账号菜单项的选择事件
func (a *App) selectAccountSelected(i *menu.Item) {
	// 创建子菜单
	subMenu := menu.NewSubMenu("切换账号")

	// 添加微信进程
	instances := a.m.wechat.Processes()
	if len(instances) > 0 {
		// 添加实例标题
		subMenu.AddItem(&menu.Item{
			Index:       0,
			Name:        "--- 微信进程 ---",
			Description: "",
			Hidden:      false,
			Selected:    nil,
		})

		// 添加实例列表
		for idx, instance := range instances {
			// 创建一个实例描述
			description := fmt.Sprintf("版本: %s 目录: %s", instance.FullVersion, instance.DataDir)

			// 标记当前选中的实例
			name := fmt.Sprintf("%s [%d]", instance.Name, instance.PID)
			if a.ctx.GetProcess() != nil && a.ctx.GetProcess().PID == instance.PID {
				name = name + " [当前]"
			}

			// 创建菜单项
			instanceItem := &menu.Item{
				Index:       idx + 1,
				Name:        name,
				Description: description,
				Hidden:      false,
				Selected: func(instance *model.Process) func(*menu.Item) {
					return func(*menu.Item) {
						// 如果是当前账号，则无需切换
						if a.ctx.GetProcess() != nil && a.ctx.GetProcess().PID == instance.PID {
							a.mainPages.RemovePage("submenu")
							a.showInfo("已经是当前账号")
							return
						}

						// 显示切换中的模态框
						modal := tview.NewModal().SetText("正在切换账号...")
						a.mainPages.AddPage("modal", modal, true, true)
						a.SetFocus(modal)

						// 在后台执行切换操作
						go func() {
							err := a.m.Switch(instance, "")

							// 在主线程中更新UI
							a.QueueUpdateDraw(func() {
								a.mainPages.RemovePage("modal")
								a.mainPages.RemovePage("submenu")

								if err != nil {
									// 切换失败
									a.showError(fmt.Errorf("切换账号失败: %v", err))
								} else {
									// 切换成功
									a.showInfo("切换账号成功")
									// 更新菜单状态
									a.updateMenuItemsState()
								}
							})
						}()
					}
				}(instance),
			}
			subMenu.AddItem(instanceItem)
		}
	}

	// 添加历史账号
	if len(a.ctx.GetAccounts()) > 0 {
		// 添加历史账号标题
		subMenu.AddItem(&menu.Item{
			Index:       100,
			Name:        "--- 历史账号 ---",
			Description: "",
			Hidden:      false,
			Selected:    nil,
		})

		// 添加历史账号列表
		idx := 101
		for account, hist := range a.ctx.GetAccounts() {
			// 创建一个账号描述
			description := fmt.Sprintf("版本: %s 目录: %s", hist.FullVersion, hist.DataDir)

			// 标记当前选中的账号
			name := account
			if name == "" {
				name = filepath.Base(hist.DataDir)
			}
			if a.ctx.GetDataDirOverride() == hist.DataDir {
				name = name + " [当前]"
			}

			// 创建菜单项
			histItem := &menu.Item{
				Index:       idx,
				Name:        name,
				Description: description,
				Hidden:      false,
				Selected: func(account string) func(*menu.Item) {
					return func(*menu.Item) {
						// 如果是当前账号，则无需切换
						if a.ctx.GetProcess() != nil && a.ctx.GetDataDirOverride() == a.ctx.GetAccounts()[account].DataDir {
							a.mainPages.RemovePage("submenu")
							a.showInfo("已经是当前账号")
							return
						}

						// 显示切换中的模态框
						modal := tview.NewModal().SetText("正在切换账号...")
						a.mainPages.AddPage("modal", modal, true, true)
						a.SetFocus(modal)

						// 在后台执行切换操作
						go func() {
							err := a.m.Switch(nil, account)

							// 在主线程中更新UI
							a.QueueUpdateDraw(func() {
								a.mainPages.RemovePage("modal")
								a.mainPages.RemovePage("submenu")

								if err != nil {
									// 切换失败
									a.showError(fmt.Errorf("切换账号失败: %v", err))
								} else {
									// 切换成功
									a.showInfo("切换账号成功")
									// 更新菜单状态
									a.updateMenuItemsState()
								}
							})
						}()
					}
				}(account),
			}
			idx++
			subMenu.AddItem(histItem)
		}
	}

	// 如果没有账号可选择
	if len(a.ctx.GetAccounts()) == 0 && len(instances) == 0 {
		subMenu.AddItem(&menu.Item{
			Index:       1,
			Name:        "无可用账号",
			Description: "未检测到微信进程或历史账号",
			Hidden:      false,
			Selected:    nil,
		})
	}

	// 显示子菜单
	a.mainPages.AddPage("submenu", subMenu, true, true)
	a.SetFocus(subMenu)
}

// showModal 显示一个模态对话框
func (a *App) showModal(text string, buttons []string, doneFunc func(buttonIndex int, buttonLabel string)) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons(buttons).
		SetDoneFunc(doneFunc)

	a.mainPages.AddPage("modal", modal, true, true)
	a.SetFocus(modal)
}

// showError 显示错误对话框
func (a *App) showError(err error) {
	a.showModal(err.Error(), []string{"OK"}, func(buttonIndex int, buttonLabel string) {
		a.mainPages.RemovePage("modal")
	})
}

// showInfo 显示信息对话框
func (a *App) showInfo(text string) {
	a.showModal(text, []string{"OK"}, func(buttonIndex int, buttonLabel string) {
		a.mainPages.RemovePage("modal")
	})
}

// showDerivedKeys 展示所有派生密钥的完整列表（可滚动，ESC 关闭）。
func (a *App) showDerivedKeys() {
	derivedKeyMap := map[string]string{}
	if a.ctx.GetProcess() != nil {
		derivedKeyMap = a.ctx.GetDerivedKeyMap()
	}

	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetBorder(true)
	tv.SetTitle(" 派生密钥列表 (ESC 关闭 / ↑↓ 滚动) ")
	tv.SetTitleAlign(tview.AlignLeft)

	if len(derivedKeyMap) == 0 {
		fmt.Fprint(tv, "\n  [gray]未获取任何派生密钥[-]\n")
	} else {
		paths := make([]string, 0, len(derivedKeyMap))
		for p := range derivedKeyMap {
			paths = append(paths, p)
		}
		sort.Strings(paths)

		fmt.Fprintf(tv, "\n  共 %d 个密钥\n\n", len(paths))
		for _, p := range paths {
			fmt.Fprintf(tv, "  [yellow]%s[-]\n  %s\n\n", p, derivedKeyMap[p])
		}
	}

	a.mainPages.AddPage("submenu", tv, true, true)
	a.SetFocus(tv)
}
