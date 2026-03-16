cmake_minimum_required(VERSION 3.17)

project(diffscope-synthesis-platform VERSION 0.0.1 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

option(APPLICATION_INSTALL "Install DiffScope Synthesis Platform" ON)

if(MSVC)
	message(ERROR "MSVC is not supported")
endif()

if(NOT DEFINED APPLICATION_NAME)
	set(APPLICATION_NAME "DiffScope Synthesis Platform")
endif()
if(NOT DEFINED APPLICATION_SEMVER)
	set(APPLICATION_SEMVER ${PROJECT_VERSION})
endif()

if(APPLICATION_INSTALL)
	include(GNUInstallDirs)
	include(CMakePackageConfigHelpers)
endif()

add_subdirectory(native)

find_program(GO_EXECUTABLE go REQUIRED)

set(APPLICATION_OUTPUT_DIR ${CMAKE_BINARY_DIR})
set(APPLICATION_EXECUTABLE ${APPLICATION_OUTPUT_DIR}/dssp${CMAKE_EXECUTABLE_SUFFIX})
set(APPLICATION_GO_STAMP ${CMAKE_BINARY_DIR}/CMakeFiles/go_build.stamp)

add_custom_command(
	OUTPUT ${APPLICATION_GO_STAMP} "force_hook.fake"
	BYPRODUCTS ${APPLICATION_EXECUTABLE}
	COMMAND ${CMAKE_COMMAND} -E make_directory ${APPLICATION_OUTPUT_DIR}
	COMMAND
		${CMAKE_COMMAND} -E env
			"CGO_ENABLED=1"
			"CGO_CPPFLAGS=-I${CMAKE_SOURCE_DIR}/native/include"
			"CGO_CXXFLAGS=-I${CMAKE_SOURCE_DIR}/native/include"
			"CGO_LDFLAGS=-L$<TARGET_FILE_DIR:dssp_native> -ldssp_native"
			"${GO_EXECUTABLE}" build
				-ldflags "-X 'diffscope-synthesis-platform/internal/appinfo.ApplicationName=${APPLICATION_NAME}' -X 'diffscope-synthesis-platform/internal/appinfo.ApplicationSemver=${APPLICATION_SEMVER}'"
				-o ${APPLICATION_EXECUTABLE}
				./cmd/dssp
	COMMAND ${CMAKE_COMMAND} -E touch ${APPLICATION_GO_STAMP}
	WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
	COMMENT "Building Go executable dssp"
	VERBATIM
)

set_source_files_properties("force_hook.fake" PROPERTIES SYMBOLIC TRUE)

add_custom_target(build_dssp ALL DEPENDS dssp_native generate_swig_bindings ${APPLICATION_GO_STAMP})

add_executable(dssp IMPORTED GLOBAL)
set_target_properties(dssp PROPERTIES IMPORTED_LOCATION ${APPLICATION_EXECUTABLE})

install(FILES $<TARGET_FILE:dssp>
	DESTINATION "${CMAKE_INSTALL_BINDIR}"
)