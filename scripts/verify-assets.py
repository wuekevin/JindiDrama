from pathlib import Path
from struct import unpack
base=Path(__file__).resolve().parents[1]/'design/reference'
for i in range(1,11):
    found=list(base.glob(f'{i:02d}-*.png'))
    assert len(found)==1, f'Missing or duplicate {i:02d}'
    data=found[0].read_bytes()
    assert data[:8]==b'\x89PNG\r\n\x1a\n', found[0]
    width,height=unpack('>II',data[16:24])
    print(f'{found[0].name}: {width}x{height}')
print('10 reference images verified')
