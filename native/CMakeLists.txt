find_package(SWIG REQUIRED)

set(APPLICATION_BINDINGS_DIR ${CMAKE_CURRENT_SOURCE_DIR})
set(APPLICATION_SWIG_INTERFACE ${CMAKE_CURRENT_SOURCE_DIR}/swig/native.i)
set(APPLICATION_SWIG_GO_FILE ${APPLICATION_BINDINGS_DIR}/native.go)
set(APPLICATION_SWIG_WRAPPER_FILE ${CMAKE_CURRENT_BINARY_DIR}/native_wrap.cxx)

file(MAKE_DIRECTORY ${APPLICATION_BINDINGS_DIR})

find_package(LangCore CONFIG REQUIRED)

get_filename_component(LANGCORE_PREFIX_DIR "${LangCore_DIR}/../.." ABSOLUTE)
set(LANGCORE_PLUGINS_DIR "${LANGCORE_PREFIX_DIR}/lib/plugins")
set(LANGCORE_G2P_PACKAGES_DIR "${LangCore_DIR}/G2pPackages")

file(GLOB_RECURSE _src src/*.cpp include/*.h)

add_library(dssp_native SHARED
	${_src}
	${APPLICATION_SWIG_WRAPPER_FILE}
)

qm_configure_target(dssp_native
	LINKS_PRIVATE
		LangCore::LangCore
		$<$<PLATFORM_ID:Windows>:dxgi.lib>
	INCLUDE_PRIVATE ${CMAKE_CURRENT_SOURCE_DIR}/include
)

if(MSVC)
	# TODO
	set_target_properties(dssp_native PROPERTIES
		WINDOWS_EXPORT_ALL_SYMBOLS ON
	)
	target_link_options(dssp_native PRIVATE "/FORCE:UNRESOLVED")
endif()

add_custom_command(TARGET dssp_native POST_BUILD
	COMMAND "${CMAKE_COMMAND}" -E copy_directory
	"${LANGCORE_PLUGINS_DIR}"
	"$<TARGET_FILE_DIR:dssp_native>/../lib/plugins"
)

add_custom_command(TARGET dssp_native POST_BUILD
	COMMAND "${CMAKE_COMMAND}" -E copy_directory
	"${LANGCORE_G2P_PACKAGES_DIR}"
	"$<TARGET_FILE_DIR:dssp_native>/../share/G2pPackages"
)

file(GLOB_RECURSE _headers include/*.h)

add_custom_command(
	OUTPUT
		${APPLICATION_SWIG_GO_FILE}
		${APPLICATION_SWIG_WRAPPER_FILE}
	COMMAND
		${SWIG_EXECUTABLE}
			-go
			-cgo
			-c++
			-intgosize 64
			-module native
			-package native
			# TODO this flag requires the next version of swig
			# -no-unique-id
			-I${CMAKE_CURRENT_SOURCE_DIR}/include
			-outdir ${APPLICATION_BINDINGS_DIR}
			-o ${APPLICATION_SWIG_WRAPPER_FILE}
			${APPLICATION_SWIG_INTERFACE}
	DEPENDS
		${APPLICATION_SWIG_INTERFACE}
		${_headers}
	COMMENT "Generating Go/C++ bindings via SWIG"
	VERBATIM
)

add_custom_target(generate_swig_bindings
	DEPENDS
		${APPLICATION_SWIG_GO_FILE}
		${APPLICATION_SWIG_WRAPPER_FILE}
)

if(APPLICATION_INSTALL)
	install(TARGETS dssp_native
		RUNTIME DESTINATION "${CMAKE_INSTALL_BINDIR}" OPTIONAL
		LIBRARY DESTINATION "${CMAKE_INSTALL_LIBDIR}" OPTIONAL
		ARCHIVE DESTINATION "${CMAKE_INSTALL_LIBDIR}" OPTIONAL
	)

	install(DIRECTORY "${LANGCORE_PLUGINS_DIR}"
		DESTINATION "${CMAKE_INSTALL_LIBDIR}"
	)

	install(DIRECTORY "${LANGCORE_G2P_PACKAGES_DIR}"
		DESTINATION "${CMAKE_INSTALL_DATADIR}"
	)
endif()
