#!/usr/bin/env python3

import sys
import subprocess
import platform
import os
import urllib.request
import shutil
import hashlib
import zipfile
import argparse

LATEST_MIDA_BINARY_LINUX = 'https://files.mida.sprai.org/mida_linux_amd64'
LATEST_MIDA_BINARY_MAC = 'https://files.mida.sprai.org/mida_darwin_amd64'
LATEST_INSTR_BROWSER_PACKAGE = 'https://files.mida.sprai.org/chromium.zip'
SHA256SUMS = 'https://files.mida.sprai.org/sha256sums.txt'


def main(args):
    if os.geteuid() != 0:
        print('Please execute with root privileges.')
        return

    # Check which platform we are running on
    OS = platform.system()
    if OS == 'Linux':
        print('Running setup for linux system')
        OS_SPECIFIC_NAME = "mida_linux_amd64"
    elif OS == 'Darwin':
        print('Running setup for OS X system')
        OS_SPECIFIC_NAME = "mida_darwin_amd64"
    else:
        print('Unsupported operating system: %s' % OS)
        return

    # Download and parse sha256sums
    sha256sums = {}
    try:
        urllib.request.urlretrieve(SHA256SUMS, '.sha256sums.txt')
    except:
        print('Unable to get SHA256 sums from web server')
        raise
    f = open('.sha256sums.txt', 'r')
    for line in f:
        pieces = line.split()
        if len(pieces) != 2:
            continue
        sha256sums[pieces[1].strip()] = pieces[0].strip()
    f.close()
    os.remove('.sha256sums.txt')

    # Check whether MIDA is installed. If not, install it.
    path = shutil.which('mida')
    if path is None:
        sys.stdout.write('Did not find existing mida installation. Installing...')
        sys.stdout.flush()
        install_mida_binary()
        sys.stdout.write('Installed!\n')
        sys.stdout.flush()
    else:
        h = hash_file(path)
        if OS_SPECIFIC_NAME in sha256sums and h == sha256sums[OS_SPECIFIC_NAME]:
            sys.stdout.write('MIDA is already up-to-date (Hash: %s)\n' % h)
            sys.stdout.flush()
        elif OS_SPECIFIC_NAME not in sha256sums:
            sys.stdout.write('Did not find a hash for this OS\n')
            sys.stdout.flush()
            return
        else:
            sys.stdout.write('Newer version of MIDA available. Installing...')
            sys.stdout.flush()
            result = install_mida_binary()
            if result is None:
                sys.stdout.write('Installed!\n')
            else:
                sys.stdout.write('Error:\n' + result + '\n')
            sys.stdout.flush()

    if args.instrumented_browser != "":
        if OS == 'Linux':
            install_instr_chromium(args.instrumented_browser)
        else:
            sys.stdout.write('Instrumented browser currently only supported on Linux\n')
            sys.stdout.flush()

    sys.stdout.write('Setup Complete.\n')
    sys.stdout.write('Type "mida help" to get started.\n')
    sys.stdout.flush()
    return


def hash_file(fname):
    HASH_BUF_SIZE = 65536
    try:
        f = open(fname, 'rb')
    except:
        print('Failed to open file %s for hashing' % fname)
        return None

    m = hashlib.sha256()
    while True:
        data = f.read(HASH_BUF_SIZE)
        if not data:
            break
        m.update(data)

    f.close()
    return m.hexdigest()


def install_mida_binary():
    sys.stdout.flush()
    if platform.system() == 'Linux':
        urllib.request.urlretrieve(LATEST_MIDA_BINARY_LINUX, '.mida.tmp')
    elif platform.system() == 'Darwin':
        urllib.request.urlretrieve(LATEST_MIDA_BINARY_MAC, '.mida.tmp')
    else:
        return 'Unsupported system type: %s' % platform.system()

    # TODO: Verify hash here or something maybe
    try:
        os.rename('.mida.tmp', '/usr/local/bin/mida')
        subprocess.call(['chmod', '0755', '/usr/local/bin/mida'])
    except:
        os.remove('.mida.tmp')
        return "Successful download but failed to install"
    return None


def is_installed(program):
    try:
        DEVNULL = open(os.devnull,'wb')
        subprocess.call(program.split(), stdout=DEVNULL, stderr=DEVNULL)
        DEVNULL.close()
    except OSError as e:
        if e.errno == os.errno.ENOENT:
            return False
        else:
            # Unknown error
            raise
    return True


def install_instr_chromium(p):
    sys.stdout.write('Downloading instrumented version of Chromium...')
    sys.stdout.flush()
    try:
        urllib.request.urlretrieve(LATEST_INSTR_BROWSER_PACKAGE, '.browser.tmp.zip')
    except:
        sys.stdout.write('Failed!')
        sys.stdout.flush()
        raise
    sys.stdout.write('Done.\n')
    sys.stdout.flush()

    u = ''
    if os.environ.get('SUDO_USER') is not None:
        u = os.environ.get('SUDO_USER')
    else:
        u = os.environ.get('USER')

    try:
        with zipfile.ZipFile('.browser.tmp.zip','r') as zip_file:
            zip_file.extractall('.browser.tmp')
    except:
        print('Failed to extract instrumented chromium from zip file')
        return
    try:
        shutil.rmtree('.browser.tmp/testing')
    except:
        print('Did not remove testing directory')

    try:
        if not os.path.isdir(os.path.join(p)):
            os.mkdir(os.path.join(p))
        os.rename('.browser.tmp/out/release-1', os.path.join(p))
        DEVNULL = open(os.devnull,'wb')
        subprocess.call(['chown', '-R', u + ':' + u, os.path.join(p)], stdout=DEVNULL, stderr=DEVNULL)
        subprocess.call(['chmod', '+x', os.path.join(p, 'chrome')], stdout=DEVNULL, stderr=DEVNULL)
        DEVNULL.close()
    except:
        sys.stdout.write('Cannot create directory: %s\n' % p)
        sys.stdout.flush()

    shutil.rmtree('.browser.tmp')
    os.remove('.browser.tmp.zip')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='MIDA setup script')
    parser.add_argument('-i', '--instrumented-browser', action="store", default="",
                        help="Path to download and install instrumented browser")
    args = parser.parse_args(sys.argv[1:])
    main(args)
