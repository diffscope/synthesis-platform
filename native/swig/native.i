%module native

%{
#include "ExecutionProviderInfo.h"
#include "LanguageService.h"
%}

%include "stdint.i"
%include "std_string.i"
%include "std_vector.i"

%include "ExecutionProviderInfo.h"
%include "LanguageService.h"

%template (ExecutionProviderInfoVector) std::vector<ExecutionProviderInfo>;
%template (DeviceInfoVector) std::vector<DeviceInfo>;