import grpc
from concurrent import futures
import vanna
from proto.vanna import vanna_pb2
from proto.vanna import vanna_pb2_grpc

class VannaServicer(vanna_pb2_grpc.VannaServiceServicer):
    def __init__(self, api_key: str):
        self.vn = vanna.OpenAI(api_key=api_key)
        
    def GenerateSQL(self, request, context):
        try:
            sql = self.vn.generate_sql(
                question=request.question,
                context={k: v for k, v in request.context.items()}
            )
            return vanna_pb2.GenerateSQLResponse(sql=sql)
        except Exception as e:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return vanna_pb2.GenerateSQLResponse()

    def ValidateSQL(self, request, context):
        try:
            is_valid = self.vn.validate_sql(request.sql)
            return vanna_pb2.ValidateSQLResponse(
                is_valid=is_valid,
                message="" if is_valid else "Invalid SQL query"
            )
        except Exception as e:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return vanna_pb2.ValidateSQLResponse()

    def ExplainSQL(self, request, context):
        try:
            explanation = self.vn.explain_sql(request.sql)
            return vanna_pb2.ExplainSQLResponse(explanation=explanation)
        except Exception as e:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return vanna_pb2.ExplainSQLResponse()

    def Train(self, request, context):
        try:
            self.vn.train(request.data)
            return vanna_pb2.TrainResponse(success=True)
        except Exception as e:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return vanna_pb2.TrainResponse(success=False, message=str(e))

def serve(api_key: str, port: int = 50051):
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    vanna_pb2_grpc.add_VannaServiceServicer_to_server(
        VannaServicer(api_key), server
    )
    server.add_insecure_port(f'[::]:{port}')
    server.start()
    print(f"Vanna service started on port {port}")
    server.wait_for_termination()

if __name__ == '__main__':
    import os
    api_key = os.getenv('VANNA_API_KEY')
    if not api_key:
        raise ValueError("VANNA_API_KEY environment variable is required")
    serve(api_key)