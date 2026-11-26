package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows/registry"
)

// cliqueDuplo diz se o nep foi aberto com dois cliques no Explorer: nesse
// caso o console foi criado só para ele (nenhum outro processo nele).
func cliqueDuplo() bool {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleProcessList")
	if proc.Find() != nil {
		return false
	}
	var ids [2]uint32
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&ids[0])), 2)
	return n == 1
}

// escondeJanela: programas chamados pelo Nepenthes (banco, atalho) não abrem
// janelas pretas.
func escondeJanela(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// esconderConsole some com a janela preta (o ícone da bandeja toma o lugar).
func esconderConsole() {
	k := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")
	u := syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
	if h, _, _ := k.Call(); h != 0 {
		u.Call(h, 0) // SW_HIDE
	}
}

func avisoErro(msg string) {
	m := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(msg)
	c, _ := syscall.UTF16PtrFromString("Nepenthes")
	m.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0x10) // MB_ICONERROR
}

//go:embed nepenthes.ico
var iconeNepenthes []byte

const chaveRun = `Software\Microsoft\Windows\CurrentVersion\Run`

func iniciarComSistema() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, chaveRun, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	_, _, err = k.GetStringValue("Nepenthes")
	return err == nil, nil
}

func mudarIniciarComSistema(sim bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, chaveRun, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !sim {
		return k.DeleteValue("Nepenthes")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue("Nepenthes", `"`+exe+`" --bandeja`)
}

// criarAtalho põe "Nepenthes" na área de trabalho (pelo PowerShell do Windows).
func criarAtalho() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	mesa := filepath.Join(home, "Desktop")
	if st, err := os.Stat(filepath.Join(home, "OneDrive", "Desktop")); err == nil && st.IsDir() {
		mesa = filepath.Join(home, "OneDrive", "Desktop")
	}
	aspas := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	ps := fmt.Sprintf(`$s=(New-Object -ComObject WScript.Shell).CreateShortcut(%s);$s.TargetPath=%s;$s.WorkingDirectory=%s;$s.Description='Nepenthes: seus sites';$s.Save()`,
		aspas(filepath.Join(mesa, "Nepenthes.lnk")), aspas(exe), aspas(filepath.Dir(exe)))
	c := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	escondeJanela(c)
	return c.Run()
}

// bandeja mostra o ícone perto do relógio, com o menu do Nepenthes, e esconde
// a janela preta. Volta quando a pessoa escolhe "Sair".
func bandeja(c *central, fechar func()) {
	sinal := make(chan os.Signal, 1)
	signal.Notify(sinal, os.Interrupt, syscall.SIGTERM)
	go func() { <-sinal; systray.Quit() }()
	systray.Run(func() {
		systray.SetIcon(iconeNepenthes)
		systray.SetTooltip("Nepenthes: seus sites")
		abrir := systray.AddMenuItem("Abrir a Central", "Abre o Nepenthes no navegador")
		pasta := systray.AddMenuItem("Pasta dos sites", "")
		systray.AddSeparator()
		ligado, _ := iniciarComSistema()
		auto := systray.AddMenuItemCheckbox("Abrir junto com o Windows", "", ligado)
		systray.AddSeparator()
		sair := systray.AddMenuItem("Sair", "Para os sites e o banco de dados")
		systray.SetOnTapped(func() { abrirNavegador("http://localhost:8000/") })
		esconderConsole()
		go func() {
			for {
				select {
				case <-abrir.ClickedCh:
					abrirNavegador("http://localhost:8000/")
				case <-pasta.ClickedCh:
					abrirPasta(c.base)
				case <-auto.ClickedCh:
					if err := mudarIniciarComSistema(!auto.Checked()); err == nil {
						if auto.Checked() {
							auto.Uncheck()
						} else {
							auto.Check()
						}
					}
				case <-sair.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, nil)
	fechar()
}
