package catalog

func suffixSpecs() []entrySpec {
	return []entrySpec{
		// v0.2 generated and test suffixes
		spec("_test.go", "source.go", RoleTest, RoleSource), spec(".pb.go", "source.go", RoleGenerated, RoleSource),
		spec(".g.dart", "source.dart", RoleGenerated, RoleSource), spec(".freezed.dart", "source.dart", RoleGenerated, RoleSource),
		spec(".generated.go", "source.go", RoleGenerated, RoleSource), spec(".gen.go", "source.go", RoleGenerated, RoleSource),
		spec(".d.ts", "source.typescript", RoleContract, RoleGenerated, RoleSource), spec(".d.mts", "source.typescript", RoleContract, RoleGenerated, RoleSource),
		spec(".d.cts", "source.typescript", RoleContract, RoleGenerated, RoleSource), spec(".test.ts", "source.typescript", RoleTest, RoleSource),
		spec(".test.tsx", "source.typescript", RoleTest, RoleSource), spec(".test.js", "source.javascript", RoleTest, RoleSource),
		spec(".test.jsx", "source.javascript", RoleTest, RoleSource), spec(".spec.ts", "source.typescript", RoleTest, RoleSource),
		spec(".spec.tsx", "source.typescript", RoleTest, RoleSource), spec(".spec.js", "source.javascript", RoleTest, RoleSource),
		spec(".spec.jsx", "source.javascript", RoleTest, RoleSource), spec(".stories.ts", "source.typescript", RoleDocument, RoleSource),
		spec(".stories.tsx", "source.typescript", RoleDocument, RoleSource), spec(".stories.js", "source.javascript", RoleDocument, RoleSource),
		spec(".stories.jsx", "source.javascript", RoleDocument, RoleSource), spec(".snap", "document.text", RoleTest, RoleGenerated),
		spec(".min.js", "source.javascript", RoleGenerated, RoleSource), spec(".min.css", "source.css", RoleGenerated, RoleSource),
		spec(".generated.ts", "source.typescript", RoleGenerated, RoleSource), spec(".generated.js", "source.javascript", RoleGenerated, RoleSource),
		spec(".gen.ts", "source.typescript", RoleGenerated, RoleSource), spec(".gen.js", "source.javascript", RoleGenerated, RoleSource),
		spec(".mock.ts", "source.typescript", RoleTest, RoleSource), spec(".mock.js", "source.javascript", RoleTest, RoleSource),
		spec(".designer.cs", "source.csharp", RoleGenerated, RoleSource), spec(".generated.cs", "source.csharp", RoleGenerated, RoleSource),

		// v0.3 language test and generated suffixes
		spec(".blade.php", "source.php", RoleSource), spec("_test.py", "source.python", RoleTest, RoleSource),
		spec(".test.py", "source.python", RoleTest, RoleSource), spec("_spec.py", "source.python", RoleTest, RoleSource),
		spec("_spec.rb", "source.ruby", RoleTest, RoleSource), spec("_test.rb", "source.ruby", RoleTest, RoleSource),
		spec(".spec.rb", "source.ruby", RoleTest, RoleSource), spec(".cy.ts", "source.typescript", RoleTest, RoleSource),
		spec(".cy.tsx", "source.typescript", RoleTest, RoleSource), spec(".cy.js", "source.javascript", RoleTest, RoleSource),
		spec(".e2e.ts", "source.typescript", RoleTest, RoleSource), spec(".e2e.js", "source.javascript", RoleTest, RoleSource),
		spec(".mocks.dart", "source.dart", RoleTest, RoleGenerated, RoleSource), spec(".generated.dart", "source.dart", RoleGenerated, RoleSource),
		spec(".pb.dart", "source.dart", RoleGenerated, RoleSource), spec(".gen.dart", "source.dart", RoleGenerated, RoleSource),
		spec("tests.cs", "source.csharp", RoleTest, RoleSource), spec("test.java", "source.java", RoleTest, RoleSource),
		spec("test.kt", "source.kotlin", RoleTest, RoleSource), spec(".test.kt", "source.kotlin", RoleTest, RoleSource),
		spec(".pb.ts", "source.typescript", RoleGenerated, RoleSource), spec(".grpc.pb.go", "source.go", RoleGenerated, RoleSource),
		spec(".mock.go", "source.go", RoleTest, RoleSource), spec("_mock.go", "source.go", RoleTest, RoleSource),
		spec(".stories.mdx", "document.markdown", RoleDocument, RoleSource), spec(".gradle.kts", "manifest.java", RoleConfig, RoleTooling),
	}
}
