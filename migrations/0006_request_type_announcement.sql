ALTER TABLE public_requests DROP CONSTRAINT IF EXISTS public_requests_request_type_check;
ALTER TABLE public_requests ADD CONSTRAINT public_requests_request_type_check CHECK (request_type IN ('announcement','suggestion','complaint','requirement','problem','idea'));
