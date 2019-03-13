#!/usr/bin/python3

import sys
import subprocess
import platform
import os
import urllib.request
import shutil
import hashlib
import zipfile

LATEST_MIDA_BINARY_LINUX = 'https://files.mida.sprai.org/mida_linux_amd64'
LATEST_MIDA_BINARY_MAC = 'https://files.mida.sprai.org/mida_darwin_amd64'
LATEST_INSTR_BROWSER_PACKAGE = 'https://files.mida.sprai.org/chromium.zip'

def main():
    if os.geteuid() != 0:
        print('Please execute with root privileges.')
        return

    # Check which platform we are running on
    OS = platform.system()
    if OS == 'Linux':
        print('Running config for linux system')
    elif OS == 'Darwin':
        print('Running config for OS X system')
    else:
        print('Unsupported operating system: %s' % OS)
        return

    # Check whether MIDA is installed
    # If not, install it
    path = shutil.which('mida')
    if path == None:
        print('Did not find existing mida installation. Installing...')
        install_mida_binary()
        print('Installed!')
    else:
        h = hash_file(path)
        print('mida already installed (hash: %s)' % h)


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
    print('Run "mida help" to get started.')
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
    sys.stdout.write('Downloading MIDA executable...')
    sys.stdout.flush()
    if platform.system() == 'Linux':
        urllib.request.urlretrieve(LATEST_MIDA_BINARY_LINUX, '.mida.tmp')
    elif platform.system() == 'Darwin':
        urllib.request.urlretrieve(LATEST_MIDA_BINARY_MAC, '.mida.tmp')
    else:
        print('Unsupported system type: %s' % platform.system())
        return

    # TODO: Verify hash here or something maybe
    os.rename('.mida.tmp', '/usr/local/bin/mida')
    subprocess.call(['chmod', '0755','/usr/local/bin/mida'])
    print('Done.')

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

