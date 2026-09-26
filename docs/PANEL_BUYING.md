# Panel acquisition and install constraints
Candidate: PeakDo U3 SE 7-inch (manual V1.0). NOT yet purchased. Cheaper panels up to
8 inches wide may be considered; every constraint below applies to any panel.

## Candidate facts (PeakDo manual V1.0)
- 7-inch; active area 155 x 87 mm; outline 172.6 x 97.7 x 9.5 mm; 152 g; CNC aluminum
  case with a flat back.
- 1920x1080 @ 60 Hz, native landscape. Mounted portrait and rotated by X (D-026); the
  OSD has no rotation setting.
- Inputs: mini-HDMI (HDMI 1.4) and a full-feature USB-C (DP 1.2). HDMI input needs
  separate power.
- Power: USB-C PD port, minimum 5 V 1 A (5 W); 3.5 W typical, 4 W maximum.
- Panel type is listed as "IPS/TN": confirm the purchased unit is IPS.
- Operating temperature 0-60 C; storage -20-60 C.
- 10-point touch reports only over the USB-C data port (port 6), which stays
  unconnected, so no touch input reaches the host (no-interaction constraint).
- Powers on automatically when its power cable is energized. Auto-sleep after 1 minute
  idle (no signal, no button, no touch), 5 minutes with "Delayed Sleep". The manual does
  not say whether a returning HDMI signal wakes it: owner task O3.
- Buttons and ports are on one long edge.

## Planned signal and power path
Video: motherboard HDMI (iGPU) -> 1 ft HDMI cable -> 24 in panel-mount HDMI to mini-HDMI
extension in a PCI slot bracket (USBFirewire RR-5S-3FPM-24G) -> panel mini-HDMI.
Power: PSU SATA power cable -> SATA-to-USB-A 5 V adapter (CRJ) -> USB-A to USB-C ->
panel PD port. The panel powers down and up with the machine.
Deferred to part arrival (owner): confirm the power-chain connectors mate end to end
(the CRJ adapter output and the USB-C adapter input are both listed as female USB-A),
and that the extension is HDMI type A female on the bracket and mini-HDMI male inside.

## Display signal path
1. The motherboard HDMI is the iGPU (amdgpu, boot_vga=1; D-023). Run owner task O1 (HDMI
   probe with any monitor) before buying if a monitor becomes available.
2. Plain HDMI or DP panels only. No DisplayLink/evdi USB displays: extra host CPU,
   driver fragility.
3. Rotation per D-026; direction set at install from the physical mounting.

## Physical / environmental
4. IPS, not OLED (burn-in on a static dashboard is a when, not an if) or TN (viewing
   angle).
5. Temperature: mount in intake airflow; measure with the side panel closed for 24 h.
6. Clearance: measure case width and glass standoff; check cable bend room and that
   connectors clear the GPUs and CPU cooler.
7. Brightness: a usable indoor range through glass (PeakDo: 500 cd/m2).
8. Mounting: double-sided tape on the flat back; removable without disassembling the
   case. The panel sits behind a 6-inch Marvin figurine; the owner positions both for
   his viewing angle.
9. Text size: at 7 inches the mockup's smallest text (19 px) is about 1.5 mm tall —
   readable at arm's length, not across a room. Accepted for v1; a larger display may
   follow after v1.

## Optional hardware
- A PWM fan hub with per-port tachs would make the eight chassis fans individually
  visible (docs/DATA.md Fans).
