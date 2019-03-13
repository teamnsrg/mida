#!/usr/bin/python3

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


def main():
    if os.geteuid() != 0:
        print('Please execute with root privileges.')
        return

    # Check which platform we are running on
    OS = platform.system()
    if OS == 'Linux':
        print('Running config for linux system')
        OS_SPECIFIC_NAME = "mida_linux_amd64"
    elif OS == 'Darwin':
        print('Running config for OS X system')
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
        return
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

    # If we are on Linux (Ubuntu), make sure Xvfb is installed
    if OS == 'Linux' and not is_installed('xvfb-run'):
        install_xvfb()

    # Make sure chromium is installed
    if OS == 'Linux' and not is_installed('chromium-browser --version'):
        install_chromium()
    else:
        print('Vanilla chromium already installed')

    if OS == 'Linux':
        install_instr_chromium()

    print('Setup Complete!')
    print('Type "mida help" to get started.')
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

def install_xvfb():
    import apt
    cache = apt.cache.Cache()
    cache.open()
    pkg = cache['xvfb']
    pkg.mark_install()
    try:
        sys.stdout.write('Installing Xvfb...')
        sys.stdout.flush()
        cache.commit()
        print('Done.')
    except Exception as e:
        print('Error installing Xvfb: %s' % e)


def install_chromium():
    import apt
    cache = apt.cache.Cache()
    cache.open()
    pkg = cache['chromium-browser']
    pkg.mark_install()
    try:
        sys.stdout.write('Installing Chromium...')
        sys.stdout.flush()
        cache.commit()
        print('Done.')
    except Exception as e:
        print('Error installing Chromium: %s' % e)

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

def install_instr_chromium():
    sys.stdout.write('Downloading instrumented version of Chromium...')
    sys.stdout.flush()
    try:
        urllib.request.urlretrieve(LATEST_INSTR_BROWSER_PACKAGE, '.browser.tmp.zip')
    except:
        print('Failed to download the latest instrumented browser package...')
        return
    print('Done.')

    p = os.path.expanduser('~')
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

    if not os.path.isdir(os.path.join(p, '.mida')):
        os.makedirs(os.path.join(p, '.mida'))

    if os.path.isdir(os.path.join(p, '.mida/browser')):
        shutil.rmtree(os.path.join(p, '.mida/browser'))

    os.rename('.browser.tmp/out/release-1', os.path.join(p, '.mida/browser'))
    shutil.rmtree('.browser.tmp')
    os.remove('.browser.tmp.zip')
    DEVNULL = open(os.devnull,'wb')
    subprocess.call(['chown', '-R', u+':'+u, os.path.join(p, '.mida')], stdout=DEVNULL, stderr=DEVNULL)
    subprocess.call(['chmod', '+x', os.path.join(p, '.mida/browser/chrome')], stdout=DEVNULL, stderr=DEVNULL)
    DEVNULL.close()


if __name__ == '__main__':
    main()

