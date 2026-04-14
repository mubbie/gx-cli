# gx.spec: PyInstaller build specification
# Usage: pyinstaller gx.spec

from PyInstaller.utils.hooks import collect_data_files, collect_submodules

block_cipher = None

hiddenimports = (
    collect_submodules('typer') +
    collect_submodules('rich') +
    collect_submodules('textual') +
    collect_submodules('click') +
    collect_submodules('gx.commands') +
    ['gx.ui.shelf_app',
     'gx.utils.git',
     'gx.utils.display',
     'gx.utils.config',
     'gx.utils.stack',
     'gx.utils.stack_render']
)

datas = (
    collect_data_files('textual') +
    collect_data_files('rich') +
    [('src/gx/VERSION', 'gx')]
)

a = Analysis(
    ['src/gx/main.py'],
    pathex=[],
    binaries=[],
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[
        'tkinter',
        'unittest',
        'xmlrpc',
        'pydoc',
        'doctest',
    ],
    noarchive=False,
)

pyz = PYZ(a.pure, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.datas,
    [],
    name='gx',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=False,
    console=True,
    icon=None,
)
