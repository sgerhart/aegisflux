Create a Dockerfile:
- python:3.11-slim
- pip install -r requirements.txt
- set PYTHONUNBUFFERED=1
- CMD ["python","-m","app.consumer"]
