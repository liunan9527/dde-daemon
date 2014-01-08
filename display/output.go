package main

import "github.com/BurntSushi/xgb/xproto"
import "github.com/BurntSushi/xgb/randr"
import "dlib/dbus"
import "fmt"

type Mode struct {
	ID     uint32
	Width  uint16
	Height uint16
	Rate   float64
}
type Output struct {
	bestMode  randr.Mode
	modes     []Mode
	rotations uint16
	crtc      randr.Crtc

	Identify randr.Output
	Name     string
	Type     uint8

	Mode         Mode
	Allocation   xproto.Rectangle
	AdjustMethod uint8

	Rotation   uint16  `access:"readwrite"`
	Reflect    uint16  `access:"readwrite"`
	Opened     bool    `access:"readwrite"`
	Brightness float64 `access:"readwrite"`
}

func (output *Output) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		"com.deepin.daemon.Display",
		fmt.Sprintf("/com/deepin/daemon/Display/Output%d", output.Identify),
		"com.deepin.daemon.Display.Output",
	}
}

func (op *Output) ListModes() []Mode {
	return op.modes
}
func (op *Output) ListRotations() []uint16 {
	return parseRotations(op.rotations)
}
func (op *Output) ListReflect() []uint16 {
	return parseReflects(op.rotations)
}

func (op *Output) SetMode(id uint32) {
	_, err := randr.SetCrtcConfig(X, op.crtc, 0, 0, op.Allocation.X, op.Allocation.Y, randr.Mode(id), op.Rotation|op.Reflect, []randr.Output{op.Identify}).Reply()
	if err != nil {
		panic(err)
	}
}

func (op *Output) SetAllocation(x, y, width, height, adjMethod int16) {
	//TODO: handle adjMethod with `width` and `height`
	_, err := randr.SetCrtcConfig(X, op.crtc, 0, 0, x, y, randr.Mode(op.Mode.ID), op.Rotation|op.Reflect, []randr.Output{op.Identify}).Reply()
	fmt.Println(err)
}

func (op *Output) OnPropertiesChanged(name string, oldv interface{}) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	switch name {
	case "Rotation":
		op.setRotation(op.Rotation)
	case "Reflect":
		op.setReflect(op.Reflect)
	case "Opened":
		op.setOpened(op.Opened)
	case "Brightness":
		op.setBrightness(op.Brightness)
	}
}

func (op *Output) Debug() string {
	return fmt.Sprintf("Allocation:%v Rotation:%d", op.Allocation, op.Rotation)
}

//-------------- internal output methods---------------------

func (op *Output) setBrightness(brightness float64) {
	if brightness < 0.01 || brightness > 1 {
		brightness = 1
	}
	gammaSize, err := randr.GetCrtcGammaSize(X, op.crtc).Reply()
	if err != nil {
		panic(fmt.Sprintf("GetCrtcGrammSize(crtc:%d) failed: %s", op.crtc, err.Error()))
	}
	red, green, blue := genGammaRamp(gammaSize.Size, brightness)
	randr.SetCrtcGamma(X, op.crtc, gammaSize.Size, red, green, blue)

}
func (op *Output) setRotation(rotation uint16) {
	v := op.Opened
	defer func() { op.setOpened(v) }()
	op.setOpened(false)

	_, err := randr.SetCrtcConfig(X, op.crtc, 0, 0, op.Allocation.X, op.Allocation.Y, op.bestMode, rotation|op.Reflect, []randr.Output{op.Identify}).Reply()
	if err != nil {
		panic(fmt.Sprintln("SetRotation:", rotation, rotation|op.Reflect, err))
	}
	op.Rotation = rotation
}

func (op *Output) setReflect(reflect uint16) {
	switch reflect {
	case 0, 16, 32, 48:
		break
	default:
		panic(fmt.Sprintf("setReflect Value%d Error", reflect))
	}

	v := op.Opened
	defer func() { op.setOpened(v) }()
	op.setOpened(false)

	_, err := randr.SetCrtcConfig(X, op.crtc, 0, 0, op.Allocation.X, op.Allocation.Y, op.bestMode, op.Rotation|reflect, []randr.Output{op.Identify}).Reply()
	op.setOpened(true)
	if err != nil {
		panic(fmt.Sprintln("SetReflect:", op.Rotation|reflect, err))
	}
	op.Reflect = reflect
}

func (op *Output) setOpened(v bool) {
	fmt.Println("SetOpened.... ", v)
	if v == true {
		oinfo, err := randr.GetOutputInfo(X, op.Identify, 0).Reply()
		if err != nil {
			panic(err)
		}
		for _, crtc := range oinfo.Crtcs {
			if isCrtcConnected(X, crtc) {
				fmt.Println("crtc:", crtc, "Is used for ", op.Name)
				continue
			}
			s, err := randr.SetCrtcConfig(X, crtc, 0, 0, op.Allocation.X, op.Allocation.Y, op.bestMode, 1, []randr.Output{op.Identify}).Reply()
			if err == nil {
				fmt.Println("Crtc:", crtc, "for", op.Name, " is ok")
				break
			}
			fmt.Println("AAAA:", s, err, crtc, op.bestMode, op.Rotation, op.Identify)
		}
	} else {
		_, err := randr.SetCrtcConfig(X, op.crtc, 0, 0, op.Allocation.X, op.Allocation.Y, 0, op.Rotation, nil).Reply()
		if err != nil {
			panic(err)
		}
	}
}

func (op *Output) updateCrtc(dpy *Display) {
	if op.crtc != 0 {
		info, err := randr.GetCrtcInfo(X, op.crtc, 0).Reply()
		if err != nil {
			panic("Opps:" + err.Error())
		}
		op.rotations = info.Rotations
		op.Rotation, op.Reflect = parseRandR(info.Rotation)

		op.Allocation = xproto.Rectangle{info.X, info.Y, info.Width, info.Height}

		op.Mode = buildMode(dpy.modes[info.Mode])
	} else {
		op.Rotation = 1
		op.Reflect = 0
		op.Allocation = xproto.Rectangle{0, 0, 0, 0}

		op.Mode = Mode{0, 0, 0, 0}
	}
	dbus.NotifyChange(op, "Allocation")
	dbus.NotifyChange(op, "Rotation")
}

func (op *Output) update(dpy *Display, info *randr.GetOutputInfoReply) {
	op.crtc = info.Crtc
	op.Opened = info.Crtc != 0
	dbus.NotifyChange(op, "Opened")
	op.bestMode = info.Modes[0]
	for _, m := range info.Modes {
		info := dpy.modes[m]
		op.modes = append(op.modes, buildMode(info))
	}
	dbus.NotifyChange(op, "Mode")
}

func NewOutput(dpy *Display, core randr.Output) *Output {
	info, err := randr.GetOutputInfo(X, core, xproto.TimeCurrentTime).Reply()
	if err != nil {
		panic(err)
		return nil
	}
	if info.Connection != randr.ConnectionConnected {
		return nil
	}

	// Nvidia driver which support Randr 1.4 will show an additional connected output which I didn't know it's exactly function. So simply filter it.
	if info.MmWidth == 0 || info.MmHeight == 0 {
		return nil
	}

	edidProp, _ := randr.GetOutputProperty(X, core, atomEDID, xproto.AtomInteger, 0, 1024, false, false).Reply()

	op := &Output{
		Identify: core,
		Name:     getOutputName(edidProp.Data, string(info.Name)),
	}
	op.update(dpy, info)
	op.updateCrtc(dpy)
	dbus.InstallOnSession(op)
	return op
}
