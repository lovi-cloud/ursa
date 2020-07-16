package gohttpd

import "text/template"

type ipxeParams struct {
	Initrd   string
	Kernel   string
	RootFS   string
	Metadata string
}

var tmpl = template.Must(template.New("iPXE").Parse(`#!ipxe

:boot_menu
menu Select the boot source
item default Default
item ipxe_shell Shell
choose --default default --timeout 3000 target && goto ${target}

:default
initrd {{ .Initrd }} || goto boot_menu
boot {{ .Kernel }} fetch={{ .RootFS }} boot=live components text console=ttyS0,115200 console=tty0 initrd=initrd.img apparmor=0 ds=nocloud-net;s={{ .Metadata }} || goto boot_menu

:ipxe_shell
shell || goto boot_menu
`))
