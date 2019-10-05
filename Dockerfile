FROM python:3.7-alpine

RUN mkdir /src

WORKDIR /src
ADD . .

RUN python setup.py install

CMD ["chromecast2mqtt"]
