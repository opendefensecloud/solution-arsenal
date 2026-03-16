FROM squidfunk/mkdocs-material

COPY ./docs/requirements.txt /requirements.txt

RUN pip install -r /requirements.txt

CMD ["serve", "--dev-addr=0.0.0.0:8000", "--livereload"]
