#!/usr/bin/python3

import sys
import subprocess
import platform
import os
import urllib.request
import shutil
import hashlib

LATEST_MIDA_BINARY = 'http://files.mida.sprai.org/mida'
TEMP_MIDA_BINARY = ".mida.tmp"


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
    if path == '':
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
    urllib.request.urlretrieve(LATEST_MIDA_BINARY, TEMP_MIDA_BINARY)
    os.rename(TEMP_MIDA_BINARY, '/usr/bin/mida')
    subprocess.call(['chmod', '0755','/usr/bin/mida'])
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
        subprocess.call([program], stdout=DEVNULL, stderr=DEVNULL)
        DEVNULL.close()
    except OSError as e:
        if e.errno == os.errno.ENOENT:
            return False
        else:
            # Unknown error
            raise
    return True



if __name__ == '__main__':
    main()
