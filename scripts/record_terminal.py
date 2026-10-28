#!/usr/bin/env python3
"""Record a real subprocess terminal session as MP4; requires Pillow and FFmpeg.

This captures live PTY output, rendered as a terminal. It does not capture a desktop.
The unmodified terminal bytes are also saved beside the video as a .log file.
"""
import argparse
import os
from pathlib import Path
import pty
import re
import select
import shutil
import subprocess
import textwrap
import time

from PIL import Image, ImageDraw, ImageFont

parser = argparse.ArgumentParser()
parser.add_argument('--out', required=True)
parser.add_argument('command', nargs=argparse.REMAINDER)
args = parser.parse_args()
if not args.command:
    parser.error('provide a command after --out filename.mp4')
encoder = os.environ.get('FFMPEG') or shutil.which('ffmpeg')
if not encoder:
    raise SystemExit('Set FFMPEG to the encoder executable path')
output = Path(args.out)
output.parent.mkdir(parents=True, exist_ok=True)
font = ImageFont.truetype('/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf', 18)
width, height, fps = 1440, 900, 5
video = subprocess.Popen([encoder, '-y', '-loglevel', 'error', '-f', 'rawvideo',
                          '-pix_fmt', 'rgb24', '-s', f'{width}x{height}', '-r', str(fps),
                          '-i', '-', '-an', '-c:v', 'libx264', '-preset', 'fast',
                          '-crf', '22', '-pix_fmt', 'yuv420p', str(output)], stdin=subprocess.PIPE)
master, slave = pty.openpty()
proc = subprocess.Popen(args.command, stdout=slave, stderr=slave, stdin=subprocess.DEVNULL)
os.close(slave)
transcript = bytearray()
started = time.monotonic()
ended = None
ansi = re.compile(r'\x1b\[[0-?]*[ -/]*[@-~]')
try:
    while True:
        frame_time = time.monotonic()
        if ended is None:
            ready, _, _ = select.select([master], [], [], 0)
            if ready:
                try:
                    transcript.extend(os.read(master, 65536))
                except OSError:
                    pass
            if proc.poll() is not None:
                # Drain all output remaining in the terminal.
                while select.select([master], [], [], 0)[0]:
                    try:
                        data = os.read(master, 65536)
                        if not data:
                            break
                        transcript.extend(data)
                    except OSError:
                        break
                ended = time.monotonic()
        screen = Image.new('RGB', (width, height), '#101821')
        draw = ImageDraw.Draw(screen)
        draw.text((24, 16), 'PJ2 | MongoDB DB Interface | Live terminal capture', font=font, fill='#79d4f5')
        lines = []
        content = ansi.sub('', transcript.decode('utf-8', errors='replace')).replace('\r', '')
        for line in content.splitlines():
            lines.extend(textwrap.wrap(line, width=124, replace_whitespace=False, drop_whitespace=False) or [''])
        for index, line in enumerate(lines[-32:]):
            draw.text((24, 58 + index * 24), line, font=font,
                      fill='#89e3ad' if line.startswith('PASS:') else '#e2eaf2')
        draw.text((24, 856), f'Elapsed {time.monotonic() - started:05.1f}s | Real command output; PTY rendered at 5 fps',
                  font=font, fill='#96a3b1')
        video.stdin.write(screen.tobytes())
        if ended is not None and time.monotonic() - ended >= 4:
            screen.save(output.with_suffix('.png'))
            break
        time.sleep(max(0, 1 / fps - (time.monotonic() - frame_time)))
finally:
    output.with_suffix('.log').write_bytes(transcript)
    os.close(master)
    video.stdin.close()
    encoder_exit = video.wait()
    if proc.poll() is None:
        proc.terminate()
        proc.wait()
if proc.returncode or encoder_exit:
    raise SystemExit(f'command={proc.returncode}; encoder={encoder_exit}')
print(f'Recorded {output}')
