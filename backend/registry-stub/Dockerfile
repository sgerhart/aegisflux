FROM python:3.11-slim
WORKDIR /app
RUN pip install flask
COPY registry.py .
EXPOSE 8090
CMD ["python","registry.py"]


