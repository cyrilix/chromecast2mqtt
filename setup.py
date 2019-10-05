from setuptools import setup, find_packages

tests_require = ['pytest'],

setup(name='chromecast2mqtt',
      version='0.1',
      description='Chromecast to mqqt bridge',
      url='https://github.com/cyrilix/chromecast2mqtt',
      author='Cyrille Nofficial',
      author_email='cynoffic@cyrilix.fr',
      license='MIT',
      entry_points={
          'console_scripts': [
              'chromecast2mqtt=chromecast2mqtt.main:main',
          ],
      },
      setup_requires=['pytest-runner'],
      install_requires=['pychromecast',
                        'paho-mqtt',
                        ],
      tests_require=tests_require,
      extras_require={'tests': tests_require},

      classifiers=[
          # How mature is this project? Common values are
          #   3 - Alpha
          #   4 - Beta
          #   5 - Production/Stable
          'Development Status :: 3 - Alpha',

          # Indicate who your project is intended for
          'Intended Audience :: Developers',

          # Pick your license as you wish (should match "license" above)
          'License :: OSI Approved :: MIT License',

          # Specify the Python versions you support here. In particular, ensure
          # that you indicate whether you support Python 2, Python 3 or both.

          'Programming Language :: Python :: 3.6',
          'Programming Language :: Python :: 3.7',
      ],
      keywords='chromecast mqtt',

      packages=find_packages(exclude=(['tests', 'venv-*'])),
      )
