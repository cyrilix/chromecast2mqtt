FROM python:3.8-alpine

RUN mkdir /src

WORKDIR /src
ADD . .

RUN python setup.py install

USER 1234

CMD ["chromecast2mqtt"]
